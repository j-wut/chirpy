package auth

import (
  "errors"
  "time"

  "github.com/golang-jwt/jwt/v5"
  "github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
  claims := jwt.RegisteredClaims{
    Issuer: "chirpy",
    IssuedAt: jwt.NewNumericDate(time.Now()),
    ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
    Subject: userID.String(),
  }


  token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
  ss, err := token.SignedString([]byte(tokenSecret))
  if err != nil {
    return "", err
  }
  return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
  token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
	  return []byte(tokenSecret), nil
  })
  if err != nil {
    return uuid.Nil, err
  } else if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
    userID, err := uuid.Parse(claims.Subject)
    if err != nil {
      return uuid.Nil, err
    }
    return userID, nil
  } else {
    return uuid.Nil, errors.New("invalid claim")
  }
}
