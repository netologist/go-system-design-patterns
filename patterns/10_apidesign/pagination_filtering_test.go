package apidesign_test

import (
	"net/url"
	"testing"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestParseListQuery_DefaultsAndBounds(t *testing.T) {
	values := url.Values{}
	values.Set("limit", "500") // Exceeds max 100 -> should clamp to 100
	values.Set("page", "3")
	values.Set("order", "asc")
	values.Set("status", "ACTIVE")

	query, err := apidesign.ParseListQuery(values, []string{"created_at", "name"}, []string{"status"})
	if err != nil {
		t.Fatalf("parse query failed: %v", err)
	}

	if query.Limit != 100 {
		t.Errorf("expected limit clamped to 100, got: %d", query.Limit)
	}
	if query.Page != 3 {
		t.Errorf("expected page 3, got: %d", query.Page)
	}
	if query.Offset() != 200 { // (3-1) * 100
		t.Errorf("expected offset 200, got: %d", query.Offset())
	}
	if query.SortOrder != apidesign.SortAsc {
		t.Errorf("expected ASC sort order, got: %s", query.SortOrder)
	}
	if query.Filters["status"] != "ACTIVE" {
		t.Errorf("expected filter status=ACTIVE, got: %s", query.Filters["status"])
	}
}

func TestParseListQuery_InvalidSortField(t *testing.T) {
	values := url.Values{}
	values.Set("sort", "unallowed_column")

	_, err := apidesign.ParseListQuery(values, []string{"created_at", "name"}, nil)
	if err == nil {
		t.Fatal("expected error on disallowed sort field, got nil")
	}
}
