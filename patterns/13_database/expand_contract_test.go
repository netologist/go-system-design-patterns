package database_test

import (
	"testing"

	database "system-design-patterns/patterns/13_database"
)

func TestExpandContractUserAdapter_ExpandPhase(t *testing.T) {
	adapter := &database.ExpandContractUserAdapter{IsContractedPhase: false}

	// 1. Dual Write
	user := database.DomainUser{
		ID:        "usr-1",
		FirstName: "Hasan",
		LastName:  "Ozgan",
	}

	row := adapter.ToDatabaseRow(user)
	if row.FullName != "Hasan Ozgan" || row.FirstName != "Hasan" || row.LastName != "Ozgan" {
		t.Errorf("dual write failed to populate all columns: %+v", row)
	}

	// 2. Read from legacy-only row (unmigrated legacy record)
	legacyRow := database.UserTableSchema{
		ID:       "usr-old",
		FullName: "Jane Smith",
	}
	readDomain := adapter.ToDomainUser(legacyRow)
	if readDomain.FirstName != "Jane" || readDomain.LastName != "Smith" {
		t.Errorf("fallback read failed: %+v", readDomain)
	}
}

func TestExpandContractUserAdapter_ContractPhase(t *testing.T) {
	adapter := &database.ExpandContractUserAdapter{IsContractedPhase: true}

	user := database.DomainUser{
		ID:        "usr-2",
		FirstName: "Alan",
		LastName:  "Turing",
	}

	row := adapter.ToDatabaseRow(user)
	if row.FullName != "" {
		t.Errorf("contract phase should not write to legacy FullName column")
	}
	if row.FirstName != "Alan" || row.LastName != "Turing" {
		t.Errorf("new columns mismatch: %+v", row)
	}
}
