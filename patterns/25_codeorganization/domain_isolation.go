package codeorganization

import (
	"errors"
	"time"
)

// Pure Domain Model: Represents core business logic, zero JSON/SQL struct tags.
type Invoice struct {
	ID          string
	TotalCents  int64
	TaxCents    int64
	Paid        bool
	IssuedAt    time.Time
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
