package persistence_test

import (
	"testing"
	"time"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestKeysetPagination_FullTraversal(t *testing.T) {
	baseTime := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	var items []persistence.Item
	for i := range 25 {
		items = append(items, persistence.Item{
			ID:        int64(i + 1),
			Title:     "Item",
			CreatedAt: baseTime.Add(time.Duration(i) * time.Minute),
		})
	}

	paginator := persistence.NewKeysetPaginator(items)

	var collected []persistence.Item
	cursor := ""
	pageSize := 10

	for {
		page, err := paginator.FetchPage(cursor, pageSize)
		if err != nil {
			t.Fatalf("fetch page error: %v", err)
		}

		collected = append(collected, page.Items...)

		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}

	if len(collected) != 25 {
		t.Fatalf("expected 25 total items collected, got: %d", len(collected))
	}

	// Verify order and uniqueness
	for i := range collected {
		if collected[i].ID != int64(i+1) {
			t.Errorf("at index %d: expected ID %d, got %d", i, i+1, collected[i].ID)
		}
	}
}
