# Safe Pagination, Filtering, and Sorting Parameter Parsing

## Overview & Definition

Building robust RESTful collection endpoints (`GET /api/v1/users?page=2&limit=50&sort=created_at&order=desc`) requires strict parsing and validation of untrusted URL query parameters.

The **Safe Pagination, Filtering, and Sorting** pattern encapsulates query extraction into a safe DTO (`ListQuery`) that:
1. **Enforces Pagination Boundaries:** Normalizes `page` (default: 1) and clamps `limit` between 1 and a maximum hard ceiling (default: 20, max: 100) to prevent denial-of-service memory allocations.
2. **Validates Whitelisted Sorting Fields:** Compares client-requested `sort` parameters against an explicit whitelist (`allowedSortFields`), preventing SQL injection and unindexed database sort crashes.
3. **Restricts Query Filters:** Filters incoming parameters against `allowedFilters`.
4. Computes relational database offsets safely via `q.Offset()`.

---

## Problem Statement

Unsanitized query parameters expose APIs to severe vulnerabilities and performance degradation:

* **Denial of Service via Unbounded Limits:** A client requesting `?limit=1000000` causes the database and Go backend to allocate hundreds of megabytes of memory to serialize millions of rows, triggering OOM crashes.
* **SQL Injection via Dynamic Order By:** Directly concatenating raw `sort` query parameters into SQL strings (`fmt.Sprintf("ORDER BY %s", query.Get("sort"))`) allows SQL injection attacks that bypass standard parameterized query placeholders.
* **Performance Degradation via Unindexed Sorting:** Allowing clients to sort on unindexed text columns forces the database to perform expensive disk-based external sorts, spiking CPU and disk I/O.

---

## Architectural Mechanism & Flow

```
                     Incoming HTTP GET /api/v1/orders?page=2&limit=500&sort=total
                                                |
                                                v
                     +------------------------------------------------------+
                     | ParseListQuery(url.Values, allowedSort, allowedFilter)|
                     +------------------------------------------------------+
                                                |
            +-----------------------------------+-----------------------------------+
            |                                   |                                   |
            v                                   v                                   v
    [Page Normalization]              [Limit Clamping]                   [Sort Whitelisting]
    - If <= 0 -> default 1            - If < 1 -> default 1              - Check against
    - Parsed integer                  - If > 100 -> clamp 100              allowedSortFields
                                                                         - Rejects invalid
                                                                           sort columns
                                                |
                                                v
                     +------------------------------------------------------+
                     | Resulting ListQuery DTO:                             |
                     |   Page: 2, Limit: 100, Offset: 100, SortBy: "total"  |
                     +------------------------------------------------------+
                                                |
                                                v
                     +------------------------------------------------------+
                     | Database Query Execution:                            |
                     |   SELECT * FROM orders ORDER BY total DESC           |
                     |   LIMIT 100 OFFSET 100                               |
                     +------------------------------------------------------+
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Enforce Strict Maximum Page Sizes:** Never allow clients to bypass the maximum `limit` ceiling (typically 50–100 items).
* **Whitelist Sort Fields and Filter Keys:** Only allow sorting on columns that possess supporting database B-Tree indexes.
* **Use Cursor-Based Pagination for Deep Datasets:** Offset-based pagination (`OFFSET 100000`) degrades in relational databases because the engine must scan and discard 100,000 index entries. For datasets with millions of records, transition from offset pagination to keyset/cursor pagination (`WHERE id > :last_seen_id LIMIT 20`).

### Common Pitfalls
* **Direct String Interpolation in SQL:** Never interpolate `r.URL.Query().Get("sort")` directly into SQL strings. Always validate against an allowlist and map to verified SQL column identifiers.
* **Missing Default Sort:** If no sort parameter is provided, always default to a deterministic indexed column (e.g. `created_at DESC` or `id DESC`) to prevent non-deterministic row ordering.

---

## Code Walkthrough & Usage

### 1. Implementation (`pagination_filtering.go`)

```go
package apidesign

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// SortOrder defines ASC or DESC sorting.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListQuery represents safe, validated pagination, filtering, and sorting parameters.
type ListQuery struct {
	Page      int
	Limit     int
	SortBy    string
	SortOrder SortOrder
	Filters   map[string]string
}

// ParseListQuery parses URL query values, applying defaults and allowed boundaries.
func ParseListQuery(values url.Values, allowedSortFields []string, allowedFilters []string) (*ListQuery, error) {
	// 1. Page
	page := 1
	if pStr := values.Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}

	// 2. Limit (Bounded between 1 and 100)
	limit := 20
	if lStr := values.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			if l < 1 {
				limit = 1
			} else if l > 100 {
				limit = 100
			} else {
				limit = l
			}
		}
	}

	// 3. Sort
	sortBy := "created_at"
	if s := values.Get("sort"); s != "" {
		allowed := false
		for _, field := range allowedSortFields {
			if strings.EqualFold(s, field) {
				sortBy = field
				allowed = true
				break
			}
		}
		if !allowed && len(allowedSortFields) > 0 {
			return nil, fmt.Errorf("invalid sort field '%s' (allowed: %v)", s, allowedSortFields)
		}
	}

	order := SortDesc
	if ordStr := strings.ToUpper(values.Get("order")); ordStr == "ASC" {
		order = SortAsc
	}

	// 4. Filters
	filters := make(map[string]string)
	for _, allowedKey := range allowedFilters {
		if val := values.Get(allowedKey); val != "" {
			filters[allowedKey] = val
		}
	}

	return &ListQuery{
		Page:      page,
		Limit:     limit,
		SortBy:    sortBy,
		SortOrder: order,
		Filters:   filters,
	}, nil
}

// Offset returns standard SQL OFFSET value.
func (q *ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
```

### 2. SQL Repository Integration

```go
func (repo *OrderRepository) ListOrders(ctx context.Context, q *ListQuery) ([]Order, error) {
    query := fmt.Sprintf(`
        SELECT id, customer_id, amount, status, created_at
        FROM orders
        WHERE ($1 = '' OR status = $1)
        ORDER BY %s %s
        LIMIT $2 OFFSET $3`, q.SortBy, q.SortOrder)

    rows, err := repo.db.QueryContext(ctx, query, q.Filters["status"], q.Limit, q.Offset())
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    // Scan records...
    return orders, nil
}
```
