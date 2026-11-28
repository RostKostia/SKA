// Package auth holds the small crypto pieces of sign-in: password hashing and random tokens.
//
// Passwords are hashed with PBKDF2-HMAC-SHA256 (RFC 8018), 600,000 iterations and a 16-byte
// random salt, the OWASP recommendation for PBKDF2. It is written out here because the backend
// keeps to the standard library (crypto/pbkdf2 only arrived in Go 1.24, and the backend
// supports 1.22). The stored form records the iteration count, so it can be raised later
// without breaking existing passwords.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Iterations is the PBKDF2 work factor for new hashes. Tests lower it to stay fast.
var Iterations = 600_000

const (
	saltLen = 16
	keyLen  = 32
	scheme  = "pbkdf2-sha256"

	MinPasswordLen = 8
	MaxPasswordLen = 200
)

var b64 = base64.RawStdEncoding

// ErrWeakPassword means the password is too short or too long.
var ErrWeakPassword = fmt.Errorf("use at least %d characters (and no more than %d)", MinPasswordLen, MaxPasswordLen)

// CheckPasswordRules returns ErrWeakPassword if the password can't be used.
func CheckPasswordRules(password string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLen || n > MaxPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// HashPassword returns "pbkdf2-sha256$<iterations>$<salt>$<hash>" (base64, no padding).
func HashPassword(password string) (string, error) {
	if err := CheckPasswordRules(password); err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := pbkdf2SHA256([]byte(password), salt, Iterations, keyLen)
	return fmt.Sprintf("%s$%d$%s$%s", scheme, Iterations, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches a hash made by HashPassword. A malformed or
// empty hash never matches.
func CheckPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != scheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > 10_000_000 {
		return false
	}
	salt, err1 := b64.DecodeString(parts[2])
	want, err2 := b64.DecodeString(parts[3])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := pbkdf2SHA256([]byte(password), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked against when an email is unknown, so a failed sign-in takes as long
// whether or not the account exists.
var dummyHash = func() string {
	salt := make([]byte, saltLen)
	return fmt.Sprintf("%s$%d$%s$%s", scheme, Iterations, b64.EncodeToString(salt), b64.EncodeToString(make([]byte, keyLen)))
}

// BurnPasswordCheck spends the same time as CheckPassword on a real hash, and returns false.
func BurnPasswordCheck(password string) bool {
	CheckPassword(dummyHash(), password)
	return false
}

// pbkdf2SHA256 is PBKDF2 (RFC 8018, section 5.2) with HMAC-SHA256 as the PRF.
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hLen := prf.Size()
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	buf := make([]byte, 4)
	u := make([]byte, hLen)
	t := make([]byte, hLen)
	for block := 1; block <= blocks; block++ {
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(buf, uint32(block))
		prf.Write(buf)
		u = prf.Sum(u[:0])
		copy(t, u)
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for i := range t {
				t[i] ^= u[i]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// NewToken returns 32 random bytes, base64url without padding (43 characters). Used for
// session tokens, invitation links, OAuth state and one-time login codes.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is what the database stores instead of a token.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// ValidToken rejects anything that can't be one of our tokens before it reaches a query.
func ValidToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}

// PKCE returns a code verifier and its S256 challenge (RFC 7636).
func PKCE() (verifier, challenge string, err error) {
	verifier, err = NewToken()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
