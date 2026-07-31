package generic

import (
	"strings"
	"testing"
)

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

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "pascal case", input: "PascalCase", want: "pascal_case"},
		{name: "camel case", input: "camelCase", want: "camel_case"},
		{name: "single word", input: "Word", want: "word"},
		{name: "already lower", input: "word", want: "word"},
		{name: "consecutive capitals", input: "ID", want: "i_d"},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToSnakeCase(tt.input); got != tt.want {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

type embeddedBase struct {
	CreatedAt string
}

type isFieldValidModel struct {
	embeddedBase
	Username string
	Email    string
}

func TestIsFieldValid(t *testing.T) {
	model := &isFieldValidModel{}

	tests := []struct {
		name  string
		field string
		want  bool
	}{
		{name: "direct field exact case", field: "Username", want: true},
		{name: "direct field snake_case", field: "username", want: true},
		{name: "embedded field found", field: "created_at", want: true},
		{name: "embedded field exact case", field: "CreatedAt", want: true},
		{name: "unknown field rejected", field: "nonexistent", want: false},
		{name: "second direct field", field: "email", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFieldValid(model, tt.field); got != tt.want {
				t.Errorf("IsFieldValid(model, %q) = %v, want %v", tt.field, got, tt.want)
			}
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	t.Run("respects requested length", func(t *testing.T) {
		for _, length := range []int{0, 1, 8, 32} {
			got := GenerateRandomString(length)
			if len(got) != length {
				t.Errorf("GenerateRandomString(%d) length = %d, want %d", length, len(got), length)
			}
			for _, c := range got {
				if !strings.ContainsRune(charset, c) {
					t.Errorf("GenerateRandomString(%d) contains unexpected character %q", length, c)
				}
			}
		}
	})

	t.Run("distinct calls are not identical", func(t *testing.T) {
		a := GenerateRandomString(24)
		b := GenerateRandomString(24)
		if a == b {
			t.Errorf("expected two random strings to differ, both were %q", a)
		}
	})
}

func TestEncryptDecryptAES256_RoundTrip(t *testing.T) {
	key := "01234567890123456789012345678901" // 32 bytes
	plaintexts := []string{"", "hello world", "a much longer plaintext string with punctuation!@#$%^&*()"}

	for _, pt := range plaintexts {
		encrypted, err := EncryptAES256(key, pt)
		if err != nil {
			t.Fatalf("EncryptAES256(%q) unexpected error: %v", pt, err)
		}
		decrypted, err := DecryptAES256(key, encrypted)
		if err != nil {
			t.Fatalf("DecryptAES256 unexpected error: %v", err)
		}
		if decrypted != pt {
			t.Errorf("round trip mismatch: got %q, want %q", decrypted, pt)
		}
	}
}

func TestEncryptDecryptAES256_ProducesDifferentCiphertextEachTime(t *testing.T) {
	key := "01234567890123456789012345678901"
	a, err := EncryptAES256(key, "same plaintext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := EncryptAES256(key, "same plaintext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Error("expected ciphertexts to differ due to random IV, but they matched")
	}
}

func TestEncryptAES256_InvalidKeyLength(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "too short", key: "shortkey"},
		{name: "too long", key: strings.Repeat("x", 33)},
		{name: "empty", key: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EncryptAES256(tt.key, "plaintext"); err == nil {
				t.Errorf("EncryptAES256 with key length %d: expected error, got nil", len(tt.key))
			}
		})
	}
}

func TestDecryptAES256_InvalidKeyLength(t *testing.T) {
	if _, err := DecryptAES256("shortkey", "irrelevant"); err == nil {
		t.Error("expected error for invalid key length, got nil")
	}
}

func TestDecryptAES256_InvalidBase64(t *testing.T) {
	key := "01234567890123456789012345678901"
	if _, err := DecryptAES256(key, "not-valid-base64!!!"); err == nil {
		t.Error("expected error for invalid base64 input, got nil")
	}
}

func TestDecryptAES256_CiphertextTooShort(t *testing.T) {
	key := "01234567890123456789012345678901"
	// Valid base64 but shorter than the AES block size.
	if _, err := DecryptAES256(key, "YWJj"); err == nil {
		t.Error("expected error for too-short ciphertext, got nil")
	}
}
