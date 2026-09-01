package di

import (
	"context"
	"errors"
	"fmt"
)

// User represents domain user entity.
type User struct {
	ID    string
	Email string
	Name  string
}

// UserRepository interface defines data persistence contract for users.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
}

// NotificationSender defines contract for dispatching notifications.
type NotificationSender interface {
	SendWelcomeEmail(ctx context.Context, email, name string) error
}

// UserService coordinates business logic with explicitly injected dependencies.
type UserService struct {
	repo     UserRepository
	notifier NotificationSender
}

// NewUserService is a constructor enforcing explicit non-nil dependencies.
func NewUserService(repo UserRepository, notifier NotificationSender) (*UserService, error) {
	if repo == nil {
		return nil, errors.New("user repository cannot be nil")
	}
	if notifier == nil {
		return nil, errors.New("notification sender cannot be nil")
	}

	return &UserService{
		repo:     repo,
		notifier: notifier,
	}, nil
}

// RegisterUser registers a user and triggers a welcome email.
func (s *UserService) RegisterUser(ctx context.Context, user *User) error {
	if user == nil || user.Email == "" {
		return errors.New("invalid user data")
	}

	if err := s.repo.Save(ctx, user); err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	if err := s.notifier.SendWelcomeEmail(ctx, user.Email, user.Name); err != nil {
		return fmt.Errorf("failed to send welcome notification: %w", err)
	}

	return nil
}
