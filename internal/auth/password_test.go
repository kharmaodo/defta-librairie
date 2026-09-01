package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "Correct-Horse-2026"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if strings.Contains(hash, password) || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("password hash has an invalid representation")
	}

	valid, err := VerifyPassword(password, hash)
	if err != nil || !valid {
		t.Fatalf("verify correct password: valid=%v err=%v", valid, err)
	}
	valid, err = VerifyPassword("Incorrect-Password", hash)
	if err != nil || valid {
		t.Fatalf("verify incorrect password: valid=%v err=%v", valid, err)
	}
}

func TestHashPasswordRejectsShortPassword(t *testing.T) {
	_, err := HashPassword("too-short")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	_, err := VerifyPassword("Correct-Horse-2026", "$argon2id$invalid")
	if !errors.Is(err, ErrInvalidPasswordHash) {
		t.Fatalf("expected ErrInvalidPasswordHash, got %v", err)
	}
}
