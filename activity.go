package gossiper

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"gorm.io/gorm"
)

// ActivitySpec is one tenant activity metric behind a GetTenantActivity RPC:
// rows of Table counted (or SumColumn summed) by TimeColumn. All fields are
// static SQL fragments written by the service, never request input.
type ActivitySpec struct {
	Key        string // stable metric key the console renders, e.g. "orders"
	Table      string // table inside the tenant schema
	TimeColumn string // timestamp the periods and weekly buckets are cut by
	SumColumn  string // optional: sum this column instead of counting rows
	Where      string // optional extra predicate, e.g. "deleted_at IS NULL"
}

// ActivityCount is one metric's result: all-time total, the last `days`
// days, the `days` before that, and `weeks` weekly buckets oldest first.
type ActivityCount struct {
	Key      string
	Total    int64
	Current  int64
	Previous int64
	Weekly   []int64
	Error    string // set when this metric could not be read (e.g. table missing)
}

var activityIdent = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// CountActivity reads every spec in namespace's schema. A failing spec (a
// product feature never used, so its table is missing on an old tenant) is
// reported on its own ActivityCount and never drops the others.
func CountActivity(ctx context.Context, db Database, namespace string, specs []ActivitySpec, days, weeks int, now time.Time) ([]ActivityCount, error) {
	if namespace == "" {
		return nil, ErrNoNamespace
	}
	days = clampInt(days, 1, 365)
	weeks = clampInt(weeks, 1, 52)
	now = now.UTC()
	curFrom := now.AddDate(0, 0, -days)
	prevFrom := now.AddDate(0, 0, -2*days)
	weeksFrom := now.AddDate(0, 0, -7*weeks)

	out := make([]ActivityCount, 0, len(specs))
	for _, spec := range specs {
		res := ActivityCount{Key: spec.Key, Weekly: make([]int64, weeks)}
		if err := spec.validate(); err != nil {
			res.Error = err.Error()
			out = append(out, res)
			continue
		}
		value := "COUNT(*)"
		if spec.SumColumn != "" {
			value = "COALESCE(SUM(" + spec.SumColumn + "), 0)"
		}
		where := "TRUE"
		if spec.Where != "" {
			where = "(" + spec.Where + ")"
		}
		err := db.WithSchemaReadOnly(ctx, namespace, func(tx *gorm.DB) error {
			var row struct{ Total, Current, Previous float64 }
			totals := fmt.Sprintf(`SELECT %[1]s AS total,
				COALESCE(%[2]s FILTER (WHERE %[3]s >= ?), 0) AS current,
				COALESCE(%[2]s FILTER (WHERE %[3]s >= ? AND %[3]s < ?), 0) AS previous
				FROM %[4]s WHERE %[5]s`, value, filterable(value), spec.TimeColumn, spec.Table, where)
			if err := tx.Raw(totals, curFrom, prevFrom, curFrom).Scan(&row).Error; err != nil {
				return err
			}
			res.Total, res.Current, res.Previous = int64(row.Total), int64(row.Current), int64(row.Previous)

			var buckets []struct {
				Week  int
				Value float64
			}
			weekly := fmt.Sprintf(`SELECT FLOOR(EXTRACT(EPOCH FROM (?::timestamptz - %[2]s)) / 604800)::int AS week, %[1]s AS value
				FROM %[3]s WHERE %[4]s AND %[2]s >= ? AND %[2]s < ? GROUP BY 1`, value, spec.TimeColumn, spec.Table, where)
			if err := tx.Raw(weekly, now, weeksFrom, now).Scan(&buckets).Error; err != nil {
				return err
			}
			for _, b := range buckets {
				if b.Week >= 0 && b.Week < weeks {
					res.Weekly[weeks-1-b.Week] = int64(b.Value)
				}
			}
			return nil
		})
		if err != nil {
			res.Error = err.Error()
		}
		out = append(out, res)
	}
	return out, nil
}

func (s ActivitySpec) validate() error {
	if s.Key == "" || !activityIdent.MatchString(s.Table) || !activityIdent.MatchString(s.TimeColumn) {
		return errors.New("invalid activity spec")
	}
	if s.SumColumn != "" && !activityIdent.MatchString(s.SumColumn) {
		return errors.New("invalid activity sum column")
	}
	return nil
}

// filterable turns the aggregate into one FILTER can attach to: COUNT(*)
// as is, COALESCE(SUM(x), 0) back to SUM(x).
func filterable(value string) string {
	if value == "COUNT(*)" {
		return value
	}
	return value[len("COALESCE(") : len(value)-len(", 0)")]
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
