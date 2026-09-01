package di_test

import (
	"context"
	"errors"
	"testing"

	di "system-design-patterns/patterns/03_di"
)

type memoryUserRepo struct {
	savedUser *di.User
	saveErr   error
}

func (m *memoryUserRepo) FindByID(ctx context.Context, id string) (*di.User, error) {
	return m.savedUser, nil
}

func (m *memoryUserRepo) Save(ctx context.Context, user *di.User) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedUser = user
	return nil
}

type memoryNotifier struct {
	lastEmail string
	sendErr   error
}

func (m *memoryNotifier) SendWelcomeEmail(ctx context.Context, email, name string) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.lastEmail = email
	return nil
}

func TestNewUserService_NilGuards(t *testing.T) {
	repo := &memoryUserRepo{}
	notifier := &memoryNotifier{}

	if _, err := di.NewUserService(nil, notifier); err == nil {
		t.Error("expected error when repo is nil")
	}

	if _, err := di.NewUserService(repo, nil); err == nil {
		t.Error("expected error when notifier is nil")
	}
}

func TestUserService_RegisterUser_Success(t *testing.T) {
	repo := &memoryUserRepo{}
	notifier := &memoryNotifier{}

	svc, err := di.NewUserService(repo, notifier)
	if err != nil {
		t.Fatalf("failed to create user service: %v", err)
	}

	user := &di.User{
		ID:    "usr-1",
		Email: "hasan@example.com",
		Name:  "Hasan",
	}

	err = svc.RegisterUser(context.Background(), user)
	if err != nil {
		t.Fatalf("expected registration to succeed, got: %v", err)
	}

	if repo.savedUser == nil || repo.savedUser.Email != "hasan@example.com" {
		t.Errorf("user was not saved to repository correctly")
	}

	if notifier.lastEmail != "hasan@example.com" {
		t.Errorf("welcome email was not sent")
	}
}

func TestUserService_RegisterUser_RepoError(t *testing.T) {
	repo := &memoryUserRepo{saveErr: errors.New("db disk full")}
	notifier := &memoryNotifier{}

	svc, _ := di.NewUserService(repo, notifier)
	err := svc.RegisterUser(context.Background(), &di.User{Email: "test@example.com"})
	if err == nil {
		t.Fatal("expected repo error, got nil")
	}
}
