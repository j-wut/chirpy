package auth

import (
    "testing"
)

func TestHash(t *testing.T) {
  password := "test"
  hashedPass, err := HashPassword(password)
  if err != nil {
    t.Errorf("error hashing password: %v", err)
  }

  err = CheckPasswordHash(password, hashedPass)
  if err != nil {
    t.Errorf("error validating hashed password: %v", err)
  }
}
