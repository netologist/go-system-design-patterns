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
