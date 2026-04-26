package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if err := CheckPassword(hash, "secret123"); err != nil {
		t.Error("CheckPassword should succeed for correct password")
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Error("CheckPassword should fail for wrong password")
	}
}

func TestCreateAndValidateSession(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	token, expiry := CreateSession(42, false, secret)
	if token == "" {
		t.Error("token should not be empty")
	}
	if expiry.Before(time.Now()) {
		t.Error("expiry should be in the future")
	}

	// Validate.
	userID, err := ValidateSession(token, secret)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if userID != 42 {
		t.Errorf("userID = %d, want 42", userID)
	}

	// Wrong secret should fail.
	_, err = ValidateSession(token, []byte("wrong-secret"))
	if err == nil {
		t.Error("expected error with wrong secret")
	}

	// Tampered token should fail.
	_, err = ValidateSession(token+"x", secret)
	if err == nil {
		t.Error("expected error with tampered token")
	}
}

func TestRememberMeSession(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	_, expiry := CreateSession(1, false, secret)
	_, longExpiry := CreateSession(1, true, secret)

	if longExpiry.Sub(expiry) < 29*24*time.Hour {
		t.Error("remember-me expiry should be ~30 days longer")
	}
}
