package messaging

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrAlreadyProcessed = errors.New("event has already been processed")
	ErrCurrentlyLocked   = errors.New("event is currently being processed by another worker")
)

type InboxStatus string

const (
	InboxProcessing InboxStatus = "PROCESSING"
	InboxCompleted  InboxStatus = "COMPLETED"
	InboxFailed     InboxStatus = "FAILED"
)

type InboxRecord struct {
	EventID     string
	Status      InboxStatus
	ProcessedAt time.Time
}

// InboxStore manages incoming event deduplication state.
type InboxStore struct {
	mu      sync.Mutex
	records map[string]*InboxRecord
}

func NewInboxStore() *InboxStore {
	return &InboxStore{
		records: make(map[string]*InboxRecord),
	}
}

// TryStartProcessing attempts to record the incoming event in PROCESSING state.
func (s *InboxStore) TryStartProcessing(ctx context.Context, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, exists := s.records[eventID]
	if exists {
		if rec.Status == InboxCompleted {
			return ErrAlreadyProcessed
		}
		if rec.Status == InboxProcessing {
			return ErrCurrentlyLocked
		}
	}

	s.records[eventID] = &InboxRecord{
		EventID:     eventID,
		Status:      InboxProcessing,
		ProcessedAt: time.Now().UTC(),
	}
	return nil
}

// MarkCompleted marks the event as successfully handled.
func (s *InboxStore) MarkCompleted(ctx context.Context, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[eventID]
	if !ok {
		return errors.New("inbox record not found")
	}

	rec.Status = InboxCompleted
	rec.ProcessedAt = time.Now().UTC()
	return nil
}

// MarkFailed marks the event as failed, allowing retry.
func (s *InboxStore) MarkFailed(ctx context.Context, eventID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec, ok := s.records[eventID]; ok {
		rec.Status = InboxFailed
	}
}
