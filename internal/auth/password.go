// Package auth hashes passwords and manages panel sessions.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the argon2id cost parameters.
//
// 64 MiB and three passes is the OWASP-recommended baseline. It costs roughly
// 50 ms per verification on modern hardware, which is irrelevant for a login
// form and expensive for an attacker with the database.
type Params struct {
	Memory      uint32 // KiB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams is what new hashes use.
func DefaultParams() Params {
	p := Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 4,
		SaltLength:  16,
		KeyLength:   32,
	}
	// Asking for more lanes than the machine has cores wastes scheduling
	// without adding cost for an attacker.
	if n := runtime.NumCPU(); n > 0 && n < int(p.Parallelism) {
		p.Parallelism = uint8(n)
	}
	return p
}

// ErrInvalidHash means the stored string is not a hash this package wrote.
var ErrInvalidHash = errors.New("auth: invalid password hash")

// HashPassword returns a PHC-formatted argon2id hash.
func HashPassword(plain string) (string, error) {
	return hashWithParams(plain, DefaultParams())
}

func hashWithParams(plain string, p Params) (string, error) {
	if plain == "" {
		return "", errors.New("auth: password must not be empty")
	}
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether plain matches encoded.
//
// A malformed hash returns an error rather than false, so a corrupt row is
// distinguishable from a wrong password.
func VerifyPassword(encoded, plain string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(plain), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash reports whether encoded was produced with weaker parameters than
// the current defaults, so a successful login can transparently upgrade it.
func NeedsRehash(encoded string) bool {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return true
	}
	def := DefaultParams()
	return p.Memory < def.Memory ||
		p.Iterations < def.Iterations ||
		p.KeyLength < def.KeyLength
}

func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", salt, key
	if len(parts) != 6 || parts[0] != "" {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return Params{}, nil, nil, fmt.Errorf("%w: algorithm %q is not argon2id", ErrInvalidHash, parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return Params{}, nil, nil, fmt.Errorf("%w: version %d is unsupported", ErrInvalidHash, version)
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if len(salt) == 0 || len(key) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))
	return p, salt, key, nil
}
