package auth

import (
	"testing"
	"time"
)

func TestJWTManagerIssueAndParse(t *testing.T) {
	m := NewJWTManager("this-is-a-test-secret-32-bytes-min", time.Minute, "cohi-api")
	token, exp, err := m.IssueAccess("user-1", "org-1", "a@b.com", "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if time.Until(exp) <= 0 {
		t.Fatal("expiry should be in the future")
	}

	claims, err := m.ParseAccess(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "user-1" || claims.OrganizationID != "org-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTManagerRejectsGarbage(t *testing.T) {
	m := NewJWTManager("this-is-a-test-secret-32-bytes-min", time.Minute, "cohi-api")
	if _, err := m.ParseAccess("not-a-jwt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshTokenHashStable(t *testing.T) {
	pepper := "pepper"
	raw, hash, err := NewRefreshToken(pepper)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("empty raw or hash")
	}
	if HashRefreshToken(raw, pepper) != hash {
		t.Fatal("hash mismatch")
	}
	if HashRefreshToken(raw+"x", pepper) == hash {
		t.Fatal("different input should not hash the same")
	}
	if HashRefreshToken(raw, "other") == hash {
		t.Fatal("different pepper should not hash the same")
	}
}
