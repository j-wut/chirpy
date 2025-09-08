package auth

import (
  "testing"
  "time"
    
  "github.com/google/uuid"
)

func TestJWT(t *testing.T) {
  userID, err := uuid.NewRandom()
  if err != nil {
    t.Errorf("error generating userID: %v", err)
  }
  password := "test"
  signedString, err := MakeJWT(userID, password, time.Second)
  if err != nil {
    t.Errorf("error generating jwt: %v", err)
  }
  
  res, err := ValidateJWT(signedString, password)
  if err != nil {
    t.Errorf("error validating jwt: %v", err)
  }

  if res != userID {
    t.Errorf("incorrect uuid from claim, expected %s, got %s", userID.String(), res.String())
  }

  time.Sleep(time.Second)

  res, err = ValidateJWT(signedString, password)
  if err == nil {
    t.Errorf("incorrect JWT validation, expected expired, but passed validation")
  }
}
