package persistence

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Document represents a persistable document with soft deletion metadata.
type Document struct {
	ID        string
	Title     string
	Content   string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func (d *Document) IsDeleted() bool {
	return d.DeletedAt != nil
}

// DocumentStore supports idempotent upserts and soft deletion.
type DocumentStore struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		docs: make(map[string]*Document),
	}
}

// IdempotentUpsert inserts or updates. Calling it repeatedly with identical data produces the same outcome.
func (s *DocumentStore) IdempotentUpsert(ctx context.Context, id, title, content string) (*Document, error) {
	if id == "" {
		return nil, errors.New("document id cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	existing, ok := s.docs[id]
	if !ok {
		doc := &Document{
			ID:        id,
			Title:     title,
			Content:   content,
			Version:   1,
			CreatedAt: now,
			UpdatedAt: now,
		}
		s.docs[id] = doc
		return doc, nil
	}

	// If no changes and not deleted, no-op idempotent return
	if existing.Title == title && existing.Content == content && !existing.IsDeleted() {
		return existing, nil
	}

	// Update existing record
	existing.Title = title
	existing.Content = content
	existing.UpdatedAt = now
	existing.DeletedAt = nil // Un-delete on update if previously soft-deleted
	existing.Version++

	return existing, nil
}

// SoftDelete marks a record as deleted without dropping the physical row.
// It is idempotent: deleting an already deleted document returns success.
func (s *DocumentStore) SoftDelete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[id]
	if !ok {
		return ErrNotFound
	}

	if doc.IsDeleted() {
		return nil // Already deleted (idempotent)
	}

	now := time.Now().UTC()
	doc.DeletedAt = &now
	doc.UpdatedAt = now
	return nil
}

// FindActive returns the document only if it has not been soft-deleted.
func (s *DocumentStore) FindActive(ctx context.Context, id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.docs[id]
	if !ok || doc.IsDeleted() {
		return nil, ErrNotFound
	}

	return doc, nil
}

// FindIncludingDeleted returns the document even if soft-deleted (for audit/admin use).
func (s *DocumentStore) FindIncludingDeleted(ctx context.Context, id string) (*Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.docs[id]
	if !ok {
		return nil, ErrNotFound
	}

	return doc, nil
}
