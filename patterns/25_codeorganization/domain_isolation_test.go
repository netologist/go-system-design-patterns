package codeorganization_test

import (
	"testing"
	"time"

	codeorganization "system-design-patterns/patterns/25_codeorganization"
)

func TestDomainIsolation_MappingPipeline(t *testing.T) {
	// 1. Raw DB row
	dbRow := codeorganization.DBInvoiceRow{
		ID:         "inv-99",
		TotalCents: 10000,
		TaxCents:   2000,
		IsPaid:     false,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	// 2. Map DB -> Domain
	domainInv, err := codeorganization.MapDBToDomain(dbRow)
	if err != nil {
		t.Fatalf("db mapping failed: %v", err)
	}

	if domainInv.GrandTotal() != 12000 {
		t.Errorf("expected grand total 12000 cents, got: %d", domainInv.GrandTotal())
	}

	// Execute domain method
	if err := domainInv.MarkAsPaid(); err != nil {
		t.Fatalf("mark as paid failed: %v", err)
	}

	// 3. Map Domain -> Public API DTO
	apiDTO := codeorganization.MapDomainToAPI(domainInv)

	if apiDTO.GrandTotal != 120.00 {
		t.Errorf("expected $120.00 in API response, got: %f", apiDTO.GrandTotal)
	}
	if !apiDTO.IsPaid {
		t.Errorf("expected is_paid true in API response")
	}
}
