package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerifyRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash has unexpected format: %q", hash)
	}
	if strings.Contains(hash, password) {
		t.Fatal("the hash contains the plaintext")
	}

	ok, err := VerifyPassword(hash, password)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("the correct password did not verify")
	}
}

func TestVerifyRejectsWrongPasswords(t *testing.T) {
	hash, err := HashPassword("the-real-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []string{
		"the-real-passwor",   // truncated
		"the-real-password ", // trailing space
		"The-Real-Password",  // different case
		"",
		"completely different",
	}
	for _, attempt := range tests {
		t.Run(attempt, func(t *testing.T) {
			ok, err := VerifyPassword(hash, attempt)
			if err != nil {
				t.Fatalf("VerifyPassword: %v", err)
			}
			if ok {
				t.Errorf("%q was accepted", attempt)
			}
		})
	}
}

func TestHashesAreSaltedUniquely(t *testing.T) {
	// Two hashes of the same password must differ, or the database would leak
	// which accounts share a password.
	a, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical; the salt is not random")
	}
	// Both must still verify.
	for i, h := range []string{a, b} {
		if ok, err := VerifyPassword(h, "same-password"); err != nil || !ok {
			t.Errorf("hash %d failed to verify: ok=%v err=%v", i, ok, err)
		}
	}
}

func TestHashRejectsEmptyPassword(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("HashPassword succeeded for an empty password")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	// A corrupt row must be distinguishable from a wrong password, so these
	// return an error rather than false.
	tests := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"not a phc string", "just-some-text"},
		{"too few fields", "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA"},
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
		{"unsupported version", "$argon2id$v=16$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
		{"unparseable params", "$argon2id$v=19$m=abc,t=3,p=4$c2FsdA$aGFzaA"},
		{"salt is not base64", "$argon2id$v=19$m=65536,t=3,p=4$!!!!$aGFzaA"},
		{"empty salt", "$argon2id$v=19$m=65536,t=3,p=4$$aGFzaA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := VerifyPassword(tt.hash, "anything")
			if err == nil {
				t.Fatal("VerifyPassword succeeded, want error")
			}
			if ok {
				t.Error("a malformed hash must never verify as true")
			}
			if !errors.Is(err, ErrInvalidHash) {
				t.Errorf("error = %v, want ErrInvalidHash", err)
			}
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	current, err := HashPassword("password-for-rehash-test")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []struct {
		name string
		hash string
		want bool
	}{
		{"current parameters", current, false},
		// Produced with weaker settings than today's defaults.
		{"weak memory", "$argon2id$v=19$m=4096,t=3,p=4$c2FsdHNhbHRzYWx0c2Fs$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY", true},
		{"few iterations", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdHNhbHRzYWx0c2Fs$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY", true},
		{"garbage counts as needing a rehash", "nonsense", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsRehash(tt.hash); got != tt.want {
				t.Errorf("NeedsRehash = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultParamsAreNotWeakened(t *testing.T) {
	// A guard against someone lowering the cost without meaning to.
	p := DefaultParams()
	if p.Memory < 64*1024 {
		t.Errorf("memory = %d KiB, want at least 65536", p.Memory)
	}
	if p.Iterations < 3 {
		t.Errorf("iterations = %d, want at least 3", p.Iterations)
	}
	if p.KeyLength < 32 {
		t.Errorf("key length = %d, want at least 32", p.KeyLength)
	}
	if p.SaltLength < 16 {
		t.Errorf("salt length = %d, want at least 16", p.SaltLength)
	}
}
