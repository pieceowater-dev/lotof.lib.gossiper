package appkit

import "testing"

func TestEnv(t *testing.T) {
	t.Setenv("AK_STR", "x")
	t.Setenv("AK_EMPTY", "")
	t.Setenv("AK_F", "0.25")
	t.Setenv("AK_BAD", "nope")
	t.Setenv("AK_I", "7")
	t.Setenv("AK_B", "true")
	if Env("AK_STR", "d") != "x" || Env("AK_MISSING_X", "d") != "d" || Env("AK_EMPTY", "d") != "" {
		t.Fatal("Env")
	}
	if EnvFloat("AK_F", 1) != 0.25 || EnvFloat("AK_BAD", 1) != 1 || EnvFloat("AK_MISSING_X", 2) != 2 {
		t.Fatal("EnvFloat")
	}
	if EnvInt("AK_I", 1) != 7 || EnvInt("AK_BAD", 1) != 1 {
		t.Fatal("EnvInt")
	}
	if !EnvBool("AK_B", false) || EnvBool("AK_STR", true) || !EnvBool("AK_MISSING_X", true) {
		t.Fatal("EnvBool")
	}
}

func TestNormalizePostgresDSN(t *testing.T) {
	if NormalizePostgresDSN("") != "" {
		t.Fatal("empty stays empty")
	}
	got := NormalizePostgresDSN("postgres://u:p@h:5432/db?sslmode=disable")
	for _, want := range []string{"statement_cache_capacity=0", "default_query_exec_mode=simple_protocol", "sslmode=disable"} {
		if !contains(got, want) {
			t.Fatalf("%q lacks %q", got, want)
		}
	}
	keep := NormalizePostgresDSN("postgres://h/db?statement_cache_capacity=5")
	if !contains(keep, "statement_cache_capacity=5") {
		t.Fatalf("explicit setting must win: %q", keep)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
