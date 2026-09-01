package security_test

import (
	"strings"
	"testing"

	security "system-design-patterns/patterns/19_security"
)

func TestUserSearchQueryBuilder_SQLInjectionSafe(t *testing.T) {
	builder := security.NewUserSearchQueryBuilder()

	// Malicious SQL injection payload in filter value
	maliciousEmail := "admin@example.com' OR '1'='1"

	filters := map[string]string{
		"email": maliciousEmail,
	}

	query, err := builder.BuildQuery(filters)
	if err != nil {
		t.Fatalf("build query failed: %v", err)
	}

	// SQL must contain parameterized placeholder $1, NOT the raw SQL string
	if strings.Contains(query.SQL, "OR '1'='1") {
		t.Fatalf("CRITICAL SQL INJECTION: raw payload concatenated into SQL: %s", query.SQL)
	}

	if !strings.Contains(query.SQL, "email = $1") {
		t.Errorf("expected parameterized placeholder $1 in SQL: %s", query.SQL)
	}

	if len(query.Args) != 1 || query.Args[0] != maliciousEmail {
		t.Errorf("arguments mismatch: %+v", query.Args)
	}
}
