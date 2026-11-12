package gossiper

import (
	"context"
	"testing"
	"time"
)

func TestActivitySpecValidate(t *testing.T) {
	ok := ActivitySpec{Key: "orders", Table: "orders", TimeColumn: "created_at", SumColumn: "total_amount"}
	if err := ok.validate(); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	for _, bad := range []ActivitySpec{
		{Table: "orders", TimeColumn: "created_at"},
		{Key: "x", Table: "orders; drop", TimeColumn: "created_at"},
		{Key: "x", Table: "orders", TimeColumn: "Created"},
		{Key: "x", Table: "orders", TimeColumn: "created_at", SumColumn: "a+b"},
	} {
		if bad.validate() == nil {
			t.Fatalf("invalid spec accepted: %+v", bad)
		}
	}
}

func TestFilterable(t *testing.T) {
	if got := filterable("COUNT(*)"); got != "COUNT(*)" {
		t.Fatalf("count: %q", got)
	}
	if got := filterable("COALESCE(SUM(total), 0)"); got != "SUM(total)" {
		t.Fatalf("sum: %q", got)
	}
}

func TestCountActivityNeedsNamespace(t *testing.T) {
	if _, err := CountActivity(context.Background(), nil, "", nil, 30, 12, time.Now()); err != ErrNoNamespace {
		t.Fatalf("want ErrNoNamespace, got %v", err)
	}
}
