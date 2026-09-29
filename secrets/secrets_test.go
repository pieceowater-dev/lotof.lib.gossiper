package secrets

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

func TestHashPIN_IsSaltedAndVerifies(t *testing.T) {
	hash, err := HashPIN("1234")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("expected a bcrypt hash, got %q", hash)
	}
	if strings.Contains(hash, "1234") {
		t.Fatal("the PIN itself must not appear in its hash")
	}
	if !VerifyPIN("1234", hash) {
		t.Fatal("the PIN must verify against its own hash")
	}
	if VerifyPIN("4321", hash) {
		t.Fatal("a different PIN must not verify")
	}

	// Salted: the same PIN hashed twice gives two different hashes, so one
	// precomputed table cannot unlock every PIN in the database.
	second, err := HashPIN("1234")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	if hash == second {
		t.Fatal("two hashes of the same PIN are identical -- no salt")
	}
	if !VerifyPIN("1234", second) {
		t.Fatal("the second hash must verify too")
	}
}

// PINs hashed before the bcrypt change are unsalted MD5. They keep working;
// the caller re-hashes them once their owner proves they know the PIN.
func TestVerifyPIN_AcceptsLegacyMD5AndFlagsIt(t *testing.T) {
	sum := md5.Sum([]byte("4321"))
	legacy := hex.EncodeToString(sum[:])

	if !IsLegacyHash(legacy) {
		t.Fatal("an MD5 digest must be recognised as legacy")
	}
	if !VerifyPIN("4321", legacy) {
		t.Fatal("a legacy PIN must still verify")
	}
	if VerifyPIN("1234", legacy) {
		t.Fatal("a wrong PIN must not verify against a legacy hash")
	}

	fresh, err := HashPIN("4321")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	if IsLegacyHash(fresh) {
		t.Fatal("a bcrypt hash must not be mistaken for a legacy one")
	}
}

func TestVerifyPIN_RejectsJunk(t *testing.T) {
	hash, err := HashPIN("1234")
	if err != nil {
		t.Fatalf("HashPIN: %v", err)
	}
	for name, pin := range map[string]string{
		"empty":       "",
		"too short":   "123",
		"too long":    "12345",
		"letters":     "12a4",
		"with spaces": " 123",
	} {
		t.Run(name, func(t *testing.T) {
			if VerifyPIN(pin, hash) {
				t.Fatalf("%q must not verify", pin)
			}
			if _, err := HashPIN(pin); err == nil {
				t.Fatalf("%q must not be hashable", pin)
			}
		})
	}
	if VerifyPIN("1234", "") {
		t.Fatal("an empty stored hash must never verify")
	}
	if VerifyPIN("1234", "not-a-hash") {
		t.Fatal("a malformed stored hash must never verify")
	}
}

func TestGeneratePIN(t *testing.T) {
	format := regexp.MustCompile(`^\d{4}$`)
	seen := map[string]int{}

	for i := 0; i < 200; i++ {
		pin := GeneratePIN()
		if !format.MatchString(pin) {
			t.Fatalf("generated %q, which is not four digits", pin)
		}
		if err := ValidatePINFormat(pin); err != nil {
			t.Fatalf("generated %q, which its own validator rejects: %v", pin, err)
		}
		seen[pin]++
	}

	// Not a randomness test -- just a guard against a constant or a counter.
	if len(seen) < 100 {
		t.Fatalf("200 generated PINs produced only %d distinct values", len(seen))
	}
}

// A check-in phrase is longer than a PIN but was hashed the same weak way.
func TestHashAndVerifyPhrase(t *testing.T) {
	hash, err := Hash("сегодня понедельник")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !Verify("сегодня понедельник", hash) {
		t.Fatal("the phrase must verify against its own hash")
	}
	if Verify("сегодня вторник", hash) {
		t.Fatal("a different phrase must not verify")
	}
	if _, err := Hash(""); err == nil {
		t.Fatal("an empty secret must not be hashable")
	}
	if Verify("", hash) || Verify("сегодня понедельник", "") {
		t.Fatal("empty input must never verify")
	}

	sum := md5.Sum([]byte("сегодня понедельник"))
	legacy := hex.EncodeToString(sum[:])
	if !IsLegacyHash(legacy) || !Verify("сегодня понедельник", legacy) {
		t.Fatal("a legacy phrase hash must still verify")
	}
}
