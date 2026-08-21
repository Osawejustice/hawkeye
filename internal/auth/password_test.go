package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "" || hash == "correct-horse-battery" {
		t.Fatal("hash should not be empty or equal to plaintext")
	}
	if err := CheckPassword(hash, "correct-horse-battery"); err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if err := CheckPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected mismatch")
	}
}
