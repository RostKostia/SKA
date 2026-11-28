package auth

import (
	"encoding/hex"
	"strings"
	"testing"
)

// Published PBKDF2-HMAC-SHA256 test vectors (the RFC 6070 inputs, SHA-256 outputs).
func TestPBKDF2Vectors(t *testing.T) {
	cases := []struct {
		pw, salt string
		iter, n  int
		want     string
	}{
		{"password", "salt", 1, 32, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"password", "salt", 2, 32, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"password", "salt", 4096, 32, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
		{"passwordPASSWORDpassword", "saltSALTsaltSALTsaltSALTsaltSALTsalt", 4096, 40,
			"348c89dbcbd32b2f32d814b8116e84cf2b17347ebc1800181c4e2a1fb8dd53e1c635518c7dac47e9"},
	}
	for _, c := range cases {
		got := hex.EncodeToString(pbkdf2SHA256([]byte(c.pw), []byte(c.salt), c.iter, c.n))
		if got != c.want {
			t.Errorf("pbkdf2(%q, %q, %d) = %s, want %s", c.pw, c.salt, c.iter, got, c.want)
		}
	}
}

func TestHashAndCheck(t *testing.T) {
	Iterations = 1000
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "pbkdf2-sha256$1000$") || strings.Contains(h, "correct") {
		t.Fatalf("hash %q", h)
	}
	if !CheckPassword(h, "correct horse") {
		t.Error("right password rejected")
	}
	for _, wrong := range []string{"correct horsE", "", "correct horse "} {
		if CheckPassword(h, wrong) {
			t.Errorf("%q accepted", wrong)
		}
	}
	h2, _ := HashPassword("correct horse")
	if h == h2 {
		t.Error("same salt twice")
	}
	for _, bad := range []string{"", "x", "pbkdf2-sha256$0$AA$AA", "pbkdf2-sha256$1000$!!$AA", "md5$1$a$b"} {
		if CheckPassword(bad, "correct horse") {
			t.Errorf("malformed hash %q matched", bad)
		}
	}
	if _, err := HashPassword("short"); err != ErrWeakPassword {
		t.Errorf("short password: %v", err)
	}
	if BurnPasswordCheck("anything") {
		t.Error("burn returned true")
	}
}

func TestTokens(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if a == b || !ValidToken(a) || len(HashToken(a)) != 32 {
		t.Fatalf("tokens %q %q", a, b)
	}
	for _, bad := range []string{"", "short", strings.Repeat("!", 43), a + "x"} {
		if ValidToken(bad) {
			t.Errorf("%q valid", bad)
		}
	}
	v, c, _ := PKCE()
	if v == c || len(c) != 43 {
		t.Errorf("pkce %q %q", v, c)
	}
}
