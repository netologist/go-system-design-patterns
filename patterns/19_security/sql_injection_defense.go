package security

import (
	"errors"
	"fmt"
	"strings"
)

// ParameterizedQuery holds safe SQL with positional placeholders and bound arguments.
type ParameterizedQuery struct {
	SQL  string
	Args []any
}

// UserSearchQueryBuilder builds safe SQL queries using parameterized placeholders.
type UserSearchQueryBuilder struct {
	allowedFields map[string]bool
}

func NewUserSearchQueryBuilder() *UserSearchQueryBuilder {
	return &UserSearchQueryBuilder{
		allowedFields: map[string]bool{
			"email":  true,
			"status": true,
			"role":   true,
		},
	}
}

// BuildQuery constructs parameterized SQL: NEVER concatenate raw user input into SQL!
func (b *UserSearchQueryBuilder) BuildQuery(filters map[string]string) (*ParameterizedQuery, error) {
	query := "SELECT id, email, status, role FROM users WHERE 1=1"
	var args []any
	argIdx := 1

	for field, val := range filters {
		if !b.allowedFields[field] {
			return nil, fmt.Errorf("disallowed filter column: %s", field)
		}
		// Positional placeholder $1, $2...
		query += fmt.Sprintf(" AND %s = $%d", field, argIdx)
		args = append(args, strings.TrimSpace(val))
		argIdx++
	}

	if len(args) == 0 {
		return nil, errors.New("at least one filter must be provided")
	}

	return &ParameterizedQuery{
		SQL:  query,
		Args: args,
	}, nil
}
