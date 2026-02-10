package database

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash should start with $argon2id$, got %q", hash)
	}

	// Hash should contain 6 dollar-sign-delimited parts.
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash should have 6 parts, got %d", len(parts))
	}
}

func TestCheckPasswordCorrect(t *testing.T) {
	password := "my-secret-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	match, err := CheckPassword(password, hash)
	if err != nil {
		t.Fatalf("CheckPassword() error: %v", err)
	}
	if !match {
		t.Error("CheckPassword() should return true for correct password")
	}
}

func TestCheckPasswordWrong(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	match, err := CheckPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("CheckPassword() error: %v", err)
	}
	if match {
		t.Error("CheckPassword() should return false for wrong password")
	}
}

func TestHashPasswordUniqueSalts(t *testing.T) {
	hash1, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() first call error: %v", err)
	}

	hash2, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() second call error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("two hashes of the same password should differ (unique salts)")
	}
}

func TestCheckPasswordInvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
	}{
		{"empty string", ""},
		{"no delimiters", "notahash"},
		{"wrong algorithm", "$bcrypt$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA"},
		{"missing parts", "$argon2id$v=19$m=65536,t=3,p=4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CheckPassword("password", tt.encoded)
			if err == nil {
				t.Error("expected error for invalid hash format")
			}
		})
	}
}

func TestCheckPasswordEmptyPassword(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	match, err := CheckPassword("", hash)
	if err != nil {
		t.Fatalf("CheckPassword() error: %v", err)
	}
	if !match {
		t.Error("CheckPassword() should return true for matching empty password")
	}

	match, err = CheckPassword("not-empty", hash)
	if err != nil {
		t.Fatalf("CheckPassword() error: %v", err)
	}
	if match {
		t.Error("CheckPassword() should return false for non-matching password")
	}
}
