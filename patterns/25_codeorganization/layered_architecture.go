package codeorganization

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Layer 1: Domain Entity (Zero framework dependencies)
type MemberAccount struct {
	ID    string
	Email string
	Tier  string
}

func (m *MemberAccount) UpgradeToPremium() error {
	if m.Tier == "PREMIUM" {
		return errors.New("member already on premium tier")
	}
	m.Tier = "PREMIUM"
	return nil
}

// Layer 2: Repository Contract (Defined near consumer)
type MemberRepository interface {
	GetMember(ctx context.Context, id string) (*MemberAccount, error)
	UpdateMember(ctx context.Context, m *MemberAccount) error
}

// Layer 3: Service Layer (Business logic orchestrator, knows nothing of HTTP)
type MemberService struct {
	repo MemberRepository
}

func NewMemberService(repo MemberRepository) *MemberService {
	return &MemberService{repo: repo}
}

func (s *MemberService) UpgradeMemberTier(ctx context.Context, id string) (*MemberAccount, error) {
	member, err := s.repo.GetMember(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := member.UpgradeToPremium(); err != nil {
		return nil, err
	}

	if err := s.repo.UpdateMember(ctx, member); err != nil {
		return nil, err
	}

	return member, nil
}

// Layer 4: HTTP Handler (Knows only HTTP request/response DTOs, delegates to Service)
type MemberHTTPHandler struct {
	service *MemberService
}

func NewMemberHTTPHandler(service *MemberService) *MemberHTTPHandler {
	return &MemberHTTPHandler{service: service}
}

func (h *MemberHTTPHandler) UpgradeTierHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/members/upgrade/")
	if id == "" {
		http.Error(w, "missing member id", http.StatusBadRequest)
		return
	}

	member, err := h.service.UpgradeMemberTier(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":   member.ID,
		"tier": member.Tier,
	})
}
