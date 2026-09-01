package auth

import "testing"

func TestRefreshTokenIsRandomAndHashed(t *testing.T) {
	first, firstHash, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("first token: %v", err)
	}
	second, secondHash, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("refresh tokens must be unique")
	}
	if first == firstHash || HashRefreshToken(first) != firstHash {
		t.Fatal("refresh token hash mismatch")
	}
}
