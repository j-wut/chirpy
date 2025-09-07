package auth

import (
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
  res, err := bcrypt.GenerateFromPassword([]byte(password), 0)
  if err != nil {
    return "", err
  }
  return string(res), nil
}

func CheckPasswordHash(password, hash string) error {
  return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
