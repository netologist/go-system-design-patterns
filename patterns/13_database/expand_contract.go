package database

import (
	"strings"
)

// UserTableSchema simulates row data during schema evolution.
type UserTableSchema struct {
	ID        string
	FullName  string // Old column (to be deprecated)
	FirstName string // New column
	LastName  string // New column
}

// ExpandContractUserAdapter adapts domain User to database row during Expand/Contract migration phases.
type ExpandContractUserAdapter struct {
	// Phase 1 (Expand): Dual-write to both old and new columns. Read from new, fallback to old.
	// Phase 2 (Contract): Read/write new columns only.
	IsContractedPhase bool
}

type DomainUser struct {
	ID        string
	FirstName string
	LastName  string
}

func (a *ExpandContractUserAdapter) ToDatabaseRow(u DomainUser) UserTableSchema {
	if a.IsContractedPhase {
		// Contract phase: Only write to new columns
		return UserTableSchema{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
		}
	}

	// Expand phase: Dual-write to both legacy and new columns for zero-downtime rolling deploys
	return UserTableSchema{
		ID:        u.ID,
		FullName:  strings.TrimSpace(u.FirstName + " " + u.LastName),
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

func (a *ExpandContractUserAdapter) ToDomainUser(row UserTableSchema) DomainUser {
	// If new columns are populated, use them
	if row.FirstName != "" || row.LastName != "" {
		return DomainUser{
			ID:        row.ID,
			FirstName: row.FirstName,
			LastName:  row.LastName,
		}
	}

	// Fallback to legacy column during transition
	parts := strings.SplitN(row.FullName, " ", 2)
	first := parts[0]
	last := ""
	if len(parts) > 1 {
		last = parts[1]
	}

	return DomainUser{
		ID:        row.ID,
		FirstName: first,
		LastName:  last,
	}
}
