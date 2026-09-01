package testingpattern

import (
	"errors"
	"strings"
)

// UserValidationService performs business validation on user creation requests.
type UserValidationService struct{}

type UserInput struct {
	Username string
	Email    string
	Age      int
}

var (
	ErrEmptyUsername = errors.New("username cannot be empty")
	ErrInvalidEmail  = errors.New("invalid email format")
	ErrUnderage      = errors.New("user must be at least 18 years old")
)

func (s *UserValidationService) Validate(input UserInput) error {
	if strings.TrimSpace(input.Username) == "" {
		return ErrEmptyUsername
	}
	if !strings.Contains(input.Email, "@") || !strings.Contains(input.Email, ".") {
		return ErrInvalidEmail
	}
	if input.Age < 18 {
		return ErrUnderage
	}
	return nil
}
