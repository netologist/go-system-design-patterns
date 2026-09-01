# Domain Isolation & DTO Mapping Pattern

## Overview & Definition
The **Domain Isolation & DTO Mapping Pattern** enforces strict structural separation between **Pure Domain Models**, **Persistence Entities (Database Rows)**, and **Transport Data Transfer Objects (API Request/Response DTOs)**.

In naive application designs, developers often use a single struct across all layers—annotating it simultaneously with JSON tags (`json:"..."`), database ORM tags (`db:"..."`, `gorm:"..."`), and validation tags (`validate:"..."`). This anti-pattern tightly couples internal database schema column names and internal business logic directly to external public API contracts, preventing the database or domain logic from evolving independently without breaking public consumers.

Domain Isolation solves this by defining distinct representations and explicit mapping functions (`MapDBToDomain`, `MapDomainToAPI`) at layer boundaries.

---

## Problem Statement
Sharing a single struct across transport, domain, and persistence layers creates severe security vulnerabilities, serialization bloat, and rigid architecture.

### Failure Scenarios Without This Pattern
- **Over-Posting / Mass Assignment Vulnerabilities:** A single struct with `json:"..."` tags deserializes directly into database models. Attackers inject fields like `"is_admin": true` or `"verified": true` in regular update payloads, escalating privileges.
- **Accidental Leakage of Sensitive Fields:** Adding an internal security field (`PasswordHash`, `InternalNotes`) to a shared model inadvertently serializes the field in public API JSON responses if `json:"-"` is omitted.
- **Breaking API Changes on Database Refactoring:** Renaming a database column (e.g., `total_cents` -> `subtotal_cents`) changes the JSON API response, breaking mobile apps and external API integrations.
- **Polluted Domain Logic with Presentation Formatting:** Formatting currency from cents to dollar decimals or parsing timestamps inside domain entities pollutes pure business models with presentation concerns.

---

## Architectural Mechanism & Flow
Each layer maintains its own specialized struct, converted explicitly at boundary crossings:

```
┌────────────────────────────────────────────────────────┐
│  Transport Layer (Public HTTP API)                     │
│  type APIInvoiceResponse struct {                      │
│      ID           string  `json:"id"`                  │
│      GrandTotal   float64 `json:"grand_total_dollars"` │
│      IsPaid       bool    `json:"is_paid"`             │
│  }                                                     │
└───────────────────────────▲────────────────────────────┘
                            │ MapDomainToAPI()
                            │ (Transforms cents -> dollars)
┌───────────────────────────┴────────────────────────────┐
│  Domain Layer (Pure Business Core)                     │
│  type Invoice struct {                                 │
│      ID         string                                 │
│      TotalCents int64                                  │
│      TaxCents   int64                                  │
│      Paid       bool                                   │
│      IssuedAt   time.Time                              │
│  }                                                     │
└───────────────────────────▲────────────────────────────┘
                            │ MapDBToDomain()
                            │ (Parses timestamps & column types)
┌───────────────────────────┴────────────────────────────┐
│  Persistence Layer (Database Storage)                  │
│  type DBInvoiceRow struct {                            │
│      ID         string `db:"id"`                       │
│      TotalCents int64  `db:"total_cents"`              │
│      TaxCents   int64  `db:"tax_cents"`                │
│      IsPaid     bool   `db:"is_paid"`                  │
│      CreatedAt  string `db:"created_at"`               │
│  }                                                     │
└────────────────────────────────────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Explicit Pure Domain Models:** Domain structs must contain zero serialization or ORM tags. They represent pure in-memory business aggregates.
- **One-Way DTO Mapping Functions:** Write explicit, idiomatic Go conversion functions (`MapDBToDomain`, `MapDomainToAPI`) rather than using complex reflection-based auto-mappers. Explicit mappers are fast ($O(1)$), type-safe, and catch missing fields at compile time.
- **Unit Representation Conversion in Mappers:** Store monetary values as integer cents (`int64`) in the domain and database; convert to decimal floating point numbers (`float64`) or formatted strings exclusively inside API response mappers.
- **Parse & Validate at Ingress Mappers:** Parse raw strings, RFC3339 timestamps, and string enums inside `MapDBToDomain` and `MapAPIToDomain`, ensuring the domain model only ever handles validated types.

### Pitfalls to Avoid
- **Using Reflection Auto-Mappers in Hot Paths:** Reflection-based mapping libraries (e.g., deep-copiers) introduce massive CPU overhead and runtime panics. Handcrafted Go functions allocate zero unnecessary memory.
- **Re-Using Response DTOs for Request Payloads:** Request DTOs (what the client sends) and Response DTOs (what the server returns) have different required fields and validation rules. Keep them separate.
- **Leaking Database Primary Keys Unnecessarily:** Exposing internal auto-incrementing integer IDs (`id: 42`) in public API DTOs allows attackers to enumerate total platform volume. Use public UUIDs or NanoIDs in API DTOs while retaining integer primary keys internally.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `domain_isolation.go` demonstrates clear separation between Domain, Persistence, and API layers:

```go
package codeorganization

import (
	"errors"
	"time"
)

// Pure Domain Model: Represents core business logic, zero JSON/SQL struct tags.
type Invoice struct {
	ID         string
	TotalCents int64
	TaxCents   int64
	Paid       bool
	IssuedAt   time.Time
}

func (inv *Invoice) GrandTotal() int64 {
	return inv.TotalCents + inv.TaxCents
}

func (inv *Invoice) MarkAsPaid() error {
	if inv.Paid {
		return errors.New("invoice is already marked as paid")
	}
	inv.Paid = true
	return nil
}

// Database Entity (Persistence Layer DTO)
type DBInvoiceRow struct {
	ID         string `db:"id"`
	TotalCents int64  `db:"total_cents"`
	TaxCents   int64  `db:"tax_cents"`
	IsPaid     bool   `db:"is_paid"`
	CreatedAt  string `db:"created_at"`
}

func MapDBToDomain(row DBInvoiceRow) (*Invoice, error) {
	issuedAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		issuedAt = time.Now()
	}
	return &Invoice{
		ID:         row.ID,
		TotalCents: row.TotalCents,
		TaxCents:   row.TaxCents,
		Paid:       row.IsPaid,
		IssuedAt:   issuedAt,
	}, nil
}

// HTTP API Response (Transport Layer DTO)
type APIInvoiceResponse struct {
	ID         string  `json:"id"`
	GrandTotal float64 `json:"grand_total_dollars"`
	IsPaid     bool    `json:"is_paid"`
}

func MapDomainToAPI(inv *Invoice) APIInvoiceResponse {
	return APIInvoiceResponse{
		ID:         inv.ID,
		GrandTotal: float64(inv.GrandTotal()) / 100.0,
		IsPaid:     inv.Paid,
	}
}
```

### Production Repository & Handler Pipeline Example

```go
// 1. Persistence Layer: Queries DB and maps to Domain
func (r *PostgresInvoiceRepo) FindByID(ctx context.Context, id string) (*codeorganization.Invoice, error) {
    var row codeorganization.DBInvoiceRow
    err := r.db.GetContext(ctx, &row, "SELECT id, total_cents, tax_cents, is_paid, created_at FROM invoices WHERE id = $1", id)
    if err != nil {
        return nil, err
    }
    return codeorganization.MapDBToDomain(row)
}

// 2. Transport Layer: Calls Service and maps Domain to JSON API Response
func (h *InvoiceHandler) GetInvoiceHandler(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    inv, err := h.service.GetInvoice(r.Context(), id)
    if err != nil {
        http.Error(w, "not found", http.StatusNotFound)
        return
    }

    // Explicit DTO conversion
    responseDTO := codeorganization.MapDomainToAPI(inv)

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(responseDTO)
}
```
