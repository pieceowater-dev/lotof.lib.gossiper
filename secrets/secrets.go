// Package secrets hashes the short secrets the platform asks people to
// remember: a cashier's manager PIN, a bonus PIN, an employee's check-in
// phrase.
//
// Three services had grown their own copy of this, all three hashing with
// unsalted MD5. A 4-digit PIN has 10 000 candidates, so an MD5 column is
// readable in the time it takes to run the loop -- and the same PIN always
// produced the same digest, so one table covered every tenant at once.
// bcrypt fixes both: a per-secret salt, and a cost that makes each guess
// expensive rather than free.
//
// Hashes written before this still verify (IsLegacyHash), so callers can
// re-hash a secret the first time its owner proves they know it instead of
// forcing everyone to pick a new one.
package secrets

import (
	"crypto/md5" // #nosec G501 -- only to verify hashes written before bcrypt
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

const hashCost = bcrypt.DefaultCost

// legacyHash matches the old unsalted MD5 digest: 32 hex characters. A
// bcrypt hash starts with "$2" and is 60 characters, so the two never
// collide.
var legacyHash = regexp.MustCompile(`^[0-9a-f]{32}$`)

var pinFormat = regexp.MustCompile(`^\d{4}$`)

// ValidatePINFormat ensures a PIN is exactly four digits.
func ValidatePINFormat(pin string) error {
	if !pinFormat.MatchString(pin) {
		return fmt.Errorf("PIN must be exactly 4 digits")
	}
	return nil
}

// HashPIN hashes a 4-digit PIN for storage.
func HashPIN(pin string) (string, error) {
	if err := ValidatePINFormat(pin); err != nil {
		return "", err
	}
	return Hash(pin)
}

// VerifyPIN reports whether pin matches hash, accepting both bcrypt and the
// legacy MD5 hashes.
func VerifyPIN(pin string, hash string) bool {
	if err := ValidatePINFormat(pin); err != nil {
		return false
	}
	return Verify(pin, hash)
}

// GeneratePIN returns a random 4-digit PIN.
//
// crypto/rand, not math/rand: a PIN drawn from a predictable stream is
// guessable without touching its hash at all.
func GeneratePIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		// crypto/rand does not fail in practice; if the platform's entropy
		// source is broken, refusing loudly beats handing out a predictable
		// PIN that looks fine.
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return fmt.Sprintf("%04d", n.Int64())
}

// Hash hashes any short secret (a phrase, a code) for storage.
func Hash(secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), hashCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(hash), nil
}

// Verify reports whether secret matches hash, accepting both bcrypt and the
// legacy MD5 hashes.
func Verify(secret string, hash string) bool {
	if secret == "" || hash == "" {
		return false
	}
	if IsLegacyHash(hash) {
		// #nosec G401 -- verifying a hash written before bcrypt; a
		// successful verification lets the caller re-hash it.
		sum := md5.Sum([]byte(secret))
		return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(hash)) == 1
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// IsLegacyHash reports whether this hash still uses the old MD5 scheme, so
// the caller can re-hash it after a successful verification.
func IsLegacyHash(hash string) bool {
	return legacyHash.MatchString(hash)
}
