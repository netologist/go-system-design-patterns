package codeorganization_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	codeorganization "system-design-patterns/patterns/25_codeorganization"
)

type memoryMemberRepo struct {
	member *codeorganization.MemberAccount
}

func (m *memoryMemberRepo) GetMember(ctx context.Context, id string) (*codeorganization.MemberAccount, error) {
	return m.member, nil
}

func (m *memoryMemberRepo) UpdateMember(ctx context.Context, member *codeorganization.MemberAccount) error {
	m.member = member
	return nil
}

func TestLayeredArchitecture_EndToEndFlow(t *testing.T) {
	// 1. Repository
	repo := &memoryMemberRepo{
		member: &codeorganization.MemberAccount{
			ID:    "mem-1",
			Email: "member@example.com",
			Tier:  "FREE",
		},
	}

	// 2. Service
	svc := codeorganization.NewMemberService(repo)

	// 3. Handler
	handler := codeorganization.NewMemberHTTPHandler(svc)

	req := httptest.NewRequest("POST", "/members/upgrade/mem-1", nil)
	w := httptest.NewRecorder()

	handler.UpgradeTierHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got: %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), `"tier":"PREMIUM"`) {
		t.Errorf("expected response tier PREMIUM, got: %s", w.Body.String())
	}

	if repo.member.Tier != "PREMIUM" {
		t.Errorf("repository state was not updated to PREMIUM")
	}
}
