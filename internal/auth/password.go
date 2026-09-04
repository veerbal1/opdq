package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Security Reason: Always check password (even for non-existent emails) to avoid leaking which emails exist.
// Quick returns (when email not found) let attackers guess valid emails by timing responses.
// By always verifying password (using DummyHash if email missing), all failed logins take similar time (e.g., >250ms).
// This prevents attackers from distinguishing valid versus invalid emails based on response time.
const DummyHash = "$2a$12$A.KkRhr3UhyggyjdBLUbke1mZvuB1H3jQA9rZCEa25VDN5RyM7kIy"
