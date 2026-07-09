package generic

import "testing"

func TestQuotePGIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple", input: "tenant_slug", want: `"tenant_slug"`},
		{name: "leading underscore", input: "_tenant", want: `"_tenant"`},
		{name: "with digits", input: "tenant123", want: `"tenant123"`},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "leading digit rejected", input: "1tenant", wantErr: true},
		{name: "sql injection via semicolon", input: "public; DROP TABLE users; --", wantErr: true},
		{name: "embedded quote rejected", input: `tenant"; --`, wantErr: true},
		{name: "whitespace rejected", input: "tenant name", wantErr: true},
		{name: "dot rejected", input: "public.users", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QuotePGIdentifier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("QuotePGIdentifier(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("QuotePGIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapePGStringLiteral(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "hunter2", want: "'hunter2'"},
		{name: "embedded single quote", input: "o'brien", want: "'o''brien'"},
		{name: "attempted breakout", input: "'; DROP TABLE users; --", want: "'''; DROP TABLE users; --'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapePGStringLiteral(tt.input); got != tt.want {
				t.Errorf("EscapePGStringLiteral(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
