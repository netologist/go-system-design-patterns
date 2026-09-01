package persistence

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Item represents a timestamped record.
type Item struct {
	ID        int64
	Title     string
	CreatedAt time.Time
}

// Cursor encodes the last-seen item's ordering fields.
type Cursor struct {
	LastID        int64
	LastTimestamp int64
}

func (c Cursor) Encode() string {
	raw := fmt.Sprintf("%d:%d", c.LastTimestamp, c.LastID)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}
	bytes, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	parts := strings.Split(string(bytes), ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}

	return &Cursor{LastTimestamp: ts, LastID: id}, nil
}

// KeysetPaginator executes efficient, stable cursor-based pagination.
type KeysetPaginator struct {
	items []Item // Sorted by CreatedAt ASC, ID ASC
}

func NewKeysetPaginator(items []Item) *KeysetPaginator {
	return &KeysetPaginator{items: items}
}

type PagedResult struct {
	Items      []Item
	NextCursor string
	HasMore    bool
}

// FetchPage fetches limit items strictly after the provided cursor.
func (p *KeysetPaginator) FetchPage(cursorStr string, limit int) (*PagedResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	cursor, err := DecodeCursor(cursorStr)
	if err != nil {
		return nil, err
	}

	var results []Item
	startIndex := 0

	if cursor != nil {
		for i, item := range p.items {
			// (item.CreatedAt, item.ID) > (cursor.LastTimestamp, cursor.LastID)
			itemTs := item.CreatedAt.UnixNano()
			if itemTs > cursor.LastTimestamp || (itemTs == cursor.LastTimestamp && item.ID > cursor.LastID) {
				startIndex = i
				break
			}
			startIndex = len(p.items) // past end if not found
		}
	}

	for i := startIndex; i < len(p.items) && len(results) < limit; i++ {
		results = append(results, p.items[i])
	}

	hasMore := startIndex+len(results) < len(p.items)
	var nextCursor string
	if len(results) > 0 && hasMore {
		last := results[len(results)-1]
		nextCursor = Cursor{
			LastTimestamp: last.CreatedAt.UnixNano(),
			LastID:        last.ID,
		}.Encode()
	}

	return &PagedResult{
		Items:      results,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
