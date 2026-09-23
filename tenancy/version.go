// Package tenancy holds the per-tenant migration bookkeeping every
// multi-tenant service needs: which tenant's schema is at which version,
// who is allowed to migrate it right now, and when to give up and retry.
//
// It exists because the same ~300 lines were copied into every service and
// then drifted into four variants -- two of them missing the DDL caching,
// one missing the panic guard around warmup (audit F1).
package tenancy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ComputeTargetVersion derives a version string from the set of AutoMigrate
// models -- their field names, types and gorm tags. Any real schema change
// changes the hash, so the migration gate notices drift on its own instead
// of depending on someone remembering to bump a constant.
func ComputeTargetVersion(models []any) string {
	signatures := make([]string, 0, len(models))
	for _, m := range models {
		signatures = append(signatures, modelSignature(m))
	}
	sort.Strings(signatures)

	h := sha256.New()
	for _, s := range signatures {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return "auto-" + hex.EncodeToString(h.Sum(nil))[:12]
}

func modelSignature(m any) string {
	t := reflect.TypeOf(m)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return "nil"
	}
	if t.Kind() != reflect.Struct {
		return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
	}

	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fields = append(fields, fmt.Sprintf("%s:%s:%s", f.Name, f.Type.String(), f.Tag))
	}
	return fmt.Sprintf("%s.%s{%s}", t.PkgPath(), t.Name(), strings.Join(fields, ","))
}
