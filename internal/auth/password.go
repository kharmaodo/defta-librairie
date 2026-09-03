package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
)

const MinPasswordLength = 12

var (
	ErrInvalidPasswordHash = errors.New("invalid password hash")
	ErrPasswordTooShort    = fmt.Errorf("password must contain at least %d characters", MinPasswordLength)
	ErrPasswordTooWeak     = errors.New("password must contain an uppercase letter, a lowercase letter, a digit and a special character")
)

type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultArgon2Params = Argon2Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func HashPassword(password string) (string, error) {
	if len([]rune(password)) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}
	var upper, lower, digit, special bool
	for _, character := range password {
		upper = upper || unicode.IsUpper(character)
		lower = lower || unicode.IsLower(character)
		digit = digit || unicode.IsDigit(character)
		special = special || (!unicode.IsLetter(character) && !unicode.IsDigit(character))
	}
	if !upper || !lower || !digit || !special {
		return "", ErrPasswordTooWeak
	}

	p := DefaultArgon2Params
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	p, salt, expected, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, actual) == 1, nil
}

func decodeHash(encodedHash string) (Argon2Params, []byte, []byte, error) {
	var p Argon2Params
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	if p.Memory < 8*1024 || p.Memory > 1024*1024 ||
		p.Iterations < 1 || p.Iterations > 10 ||
		p.Parallelism < 1 || p.Parallelism > 16 {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return p, nil, nil, ErrInvalidPasswordHash
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(expected) < 16 {
		return p, nil, nil, ErrInvalidPasswordHash
	}

	return p, salt, expected, nil
}
