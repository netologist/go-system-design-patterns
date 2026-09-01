# Layered Architecture Pattern (Clean / Hexagonal Layer Separation)

## Overview & Definition
The **Layered Architecture Pattern** (also known as Clean Architecture, Ports & Adapters, or Hexagonal Architecture) organizes Go backend code into distinct, decoupled architectural tiers with strict unidirectional dependency rules.

In an idiomatic Go layered architecture, dependencies point strictly inward toward the core domain logic:
1. **Layer 1: Domain Entities (`MemberAccount`):** Pure Go structs encapsulating core domain models and business invariants. Zero dependencies on databases, HTTP frameworks, or external packages.
2. **Layer 2: Repository Contracts (`MemberRepository`):** Small, focused interfaces defined alongside the business consumers that need them ("accept interfaces, return structs").
3. **Layer 3: Application Services (`MemberService`):** Orchestrators of business use-cases, coordinating repositories, domain entities, and domain events. Completely agnostic of transport protocols (HTTP/gRPC/CLI).
4. **Layer 4: Transport Adapters (`MemberHTTPHandler`):** Edge adapters responsible for parsing transport-specific requests (HTTP JSON, URL parameters, gRPC messages), invoking domain services, and translating domain outcomes into HTTP status codes and responses.

---

## Problem Statement
Monolithic, tightly-coupled codebases that mix SQL queries, HTTP request handling, and business rules inside single controller functions become unmaintainable and impossible to test.

### Failure Scenarios Without This Pattern
- **Fat Controllers with Database Invasions:** HTTP handlers executing raw SQL queries directly inside `w.Write()` loops. Modifying the database schema requires rewriting HTTP routing logic.
- **Untestable Business Logic:** Business logic intertwined with `*http.Request` and `http.ResponseWriter` cannot be unit-tested without spinning up mock HTTP servers and network sockets.
- **Transport Lock-In:** Rewriting an HTTP endpoint to support a gRPC API, CLI command, or asynchronous queue worker requires duplicating core business rules.
- **Circular Package Dependencies (`import cycle not allowed`):** High coupling between packages leads to circular import compilation errors in Go.

---

## Architectural Mechanism & Flow
Dependencies flow strictly from outer transport layers down to the core domain, inverted via repository interfaces:

```
┌────────────────────────────────────────────────────────┐
│  Transport Layer: MemberHTTPHandler                    │
│  - Parses HTTP JSON / URL Path                         │
│  - Maps HTTP Status Codes (200, 400, 422)              │
└───────────────────────────┬────────────────────────────┘
                            │ (Calls Service)
                            ▼
┌────────────────────────────────────────────────────────┐
│  Service Layer: MemberService                          │
│  - Orchestrates business use case: UpgradeMemberTier   │
│  - Depends on abstract MemberRepository interface      │
└─────────────┬────────────────────────────┬─────────────┘
              │ (Invokes Entity)           │ (Calls Interface)
              ▼                            ▼
┌───────────────────────────┐  ┌─────────────────────────┐
│ Domain Entity:            │  │ Interface Contract:     │
│ MemberAccount             │  │ MemberRepository        │
│ - Pure business rules     │  │ - GetMember()           │
│ - UpgradeToPremium()      │  │ - UpdateMember()        │
└───────────────────────────┘  └───────────▲─────────────┘
                                           │ (Implements)
                               ┌───────────┴─────────────┐
                               │ Persistence Adapter:    │
                               │ PostgresMemberRepo      │
                               └─────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Define Interfaces at the Consumer Boundary:** Declare repository interfaces in the package where they are consumed (the service package), not in the package that implements them.
- **Keep Domain Models Free of External Tags:** Keep pure domain structs free of framework-specific annotations (`gorm:"..."`, `json:"..."`, `db:"..."`).
- **Use Dependency Injection via Constructors:** Inject repository interfaces into services (`NewMemberService(repo)`) and services into handlers (`NewMemberHTTPHandler(svc)`), enabling trivial mocking in unit tests.
- **Fail Fast with Domain Errors:** Return clear domain errors from entities and services; let the transport layer map them to HTTP 400, 404, 409, or 500 status codes.

### Pitfalls to Avoid
- **Passing `*http.Request` or `context.WithValue` into Services:** The service layer should only receive standard `context.Context` and explicit domain parameters.
- **Leaking Database Connection Types into Domain Services:** Never accept `*sql.DB` or `*sql.Tx` in domain entity methods.
- **Over-Abstraction on Trivial CRUD:** Avoid introducing dozens of layers and interfaces for trivial lookup tables that possess no domain invariants.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `layered_architecture.go` demonstrates clean separation of concerns:

```go
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
```

### Production Application Wiring Example

```go
func main() {
    // 1. Initialize Persistence Adapter
    dbPool := connectPostgres()
    memberRepo := postgres.NewMemberRepository(dbPool)

    // 2. Initialize Service Layer
    memberService := codeorganization.NewMemberService(memberRepo)

    // 3. Initialize Transport Adapter
    httpHandler := codeorganization.NewMemberHTTPHandler(memberService)

    // 4. Mount Routes
    mux := http.NewServeMux()
    mux.HandleFunc("POST /members/upgrade/", httpHandler.UpgradeTierHandler)

    log.Println("Server running on :8080...")
    _ = http.ListenAndServe(":8080", mux)
}
```
