# SQL Injection Defense Pattern (Parameterized Query Builder)

## 1. Overview & Concept
The **SQL Injection Defense Pattern** protects relational databases by strictly decoupling SQL command semantics (the structure, grammar, and execution plan of the query) from dynamic user-provided data values. Instead of constructing SQL strings via string concatenation, formatting (`fmt.Sprintf`), or templating, this pattern enforces the use of **parameterized queries** with positional placeholders (`$1, $2` in PostgreSQL, `?` in MySQL/SQLite) and explicit column allowlisting.

In production Go backend applications interacting with databases via `database/sql`, `pgx`, or `sqlx`, dynamic queries (such as search filters, faceted navigation, dynamic sorting, and multi-field filtering) must be safely constructed without exposing dynamic column names or values to injection vectors.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
SQL Injection (CWE-89) remains one of the most severe application security risks, consistently enabling unauthorized database access, privilege escalation, data destruction, and full data exfiltration.

### Failure Scenarios Without This Pattern
- **Authentication Bypass & Unauthorized Data Access:** Attackers injecting payloads like `' OR '1'='1` or `admin' --` into unsanitized queries bypass authentication checks or dump complete user tables.
- **Arbitrary Data Destruction & Modification:** Dynamic SQL concatenation allows attackers to terminate existing queries and append destructive statements (e.g., `; DROP TABLE orders; --` or `; UPDATE users SET role='superadmin' WHERE id=1;`).
- **Dynamic Filter Vulnerabilities (Identifier Injection):** While values can be safely bound with placeholders, column names in `ORDER BY` or `WHERE` clauses cannot be parameterized in standard SQL drivers. If dynamic columns are concatenated directly, attackers execute subqueries or sleep commands through column names.
- **Second-Order SQL Injection:** Untrusted strings saved without validation execute maliciously when retrieved and concatenated into subsequent dynamic queries.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `UserSearchQueryBuilder` combines **column allowlisting** for dynamic query clauses with **positional parameter binding** for all dynamic filter arguments:

```
 [ Dynamic Filters: { "status": "active", "role": "admin" } ]
                             │
                             ▼
              ┌───────────────────────────────┐
              │    UserSearchQueryBuilder     │
              └──────────────┬────────────────┘
                             │
             For each requested filter column:
                             │
              ┌──────────────┴──────────────┐
              │ Is column in allowed list?  │
              └──────────────┬──────────────┘
                    │                 │
                   [No]             [Yes]
                    │                 │
                    ▼                 ▼
          [Reject with Error]   [Append Positional Placeholder]
                                [ e.g. " AND status = $1" ]
                                      │
                                      ▼
                                [Append Bound Value to Args]
                                      │
                                      ▼
             ┌─────────────────────────────────────────────────┐
             │            ParameterizedQuery Struct            │
             │ SQL: "SELECT ... WHERE 1=1 AND status = $1..."  │
             │ Args: ["active", "admin"]                       │
             └────────────────────────┬────────────────────────┘
                                      │
                                      ▼
                         [ Execute via db.QueryContext ]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Always Use Prepared Placeholders for Values:** Never concatenate values directly into SQL strings. Use driver-native placeholders (`$1`, `$2` for Postgres, `?` for MySQL/SQLite).
- **Allowlist Dynamic Identifiers:** Column names, table names, and sort directions (`ASC`/`DESC`) cannot be parameterized by SQL drivers. Always validate them against hardcoded sets/maps before inclusion.
- **Normalize and Trim Inputs:** Clean and trim input strings before binding them to parameters.
- **Apply Principle of Least Privilege:** Configure database users with minimal necessary permissions (e.g., read-only users for analytical queries, no DDL permissions for application runtime users).
- **Use Context-Aware Query Execution:** Always pass `context.Context` with bounded timeouts to `db.QueryContext` and `db.ExecContext` to prevent slow-query DoS attacks.

### Pitfalls to Avoid
- **False Sense of Security from ORMs:** Naively passing unvetted user input into ORM raw clauses (e.g., `db.Where(fmt.Sprintf("name = '%s'", input))`) bypasses ORM safety entirely.
- **String Substitution in Stored Procedures:** Dynamic SQL inside stored procedures (`EXECUTE IMMEDIATE` or `sp_executesql`) can still be vulnerable if parameters are concatenated within PL/pgSQL or T-SQL routines.
- **Ignoring Dynamic Ordering/Pagination:** Developers often parameterize `WHERE` clauses but concatenate `ORDER BY` fields directly from query strings.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/sql_injection_defense.go`
```go
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
```

### Production Database Execution Example
```go
func SearchUsers(ctx context.Context, db *sql.DB, filters map[string]string) ([]User, error) {
    builder := security.NewUserSearchQueryBuilder()
    
    // Safely generate SQL and bound arguments
    pq, err := builder.BuildQuery(filters)
    if err != nil {
        return nil, fmt.Errorf("invalid search filters: %w", err)
    }

    // Execute with context and bound parameters
    rows, err := db.QueryContext(ctx, pq.SQL, pq.Args...)
    if err != nil {
        return nil, fmt.Errorf("database query error: %w", err)
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Email, &u.Status, &u.Role); err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, rows.Err()
}
```
