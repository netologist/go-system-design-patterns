package distributed

import (
	"context"
	"sync"
)

// Record represents a domain entity with revision/version.
type Record struct {
	ID      string
	Version int64
	Data    string
}

// ReconciliationReport summarizes detected drift between source and target systems.
type ReconciliationReport struct {
	MissingInTarget   []string
	MismatchInTarget  []string
	OrphansInTarget   []string
	RepairedCount     int
}

// ReconciliationStore represents storage participating in reconciliation.
type ReconciliationStore interface {
	GetAll(ctx context.Context) (map[string]Record, error)
	Upsert(ctx context.Context, r Record) error
	Delete(ctx context.Context, id string) error
}

// MemoryReconciliationStore thread-safe memory store.
type MemoryReconciliationStore struct {
	mu    sync.RWMutex
	store map[string]Record
}

func NewMemoryReconciliationStore() *MemoryReconciliationStore {
	return &MemoryReconciliationStore{
		store: make(map[string]Record),
	}
}

func (s *MemoryReconciliationStore) GetAll(ctx context.Context) (map[string]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]Record, len(s.store))
	for k, v := range s.store {
		res[k] = v
	}
	return res, nil
}

func (s *MemoryReconciliationStore) Upsert(ctx context.Context, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[r.ID] = r
	return nil
}

func (s *MemoryReconciliationStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, id)
	return nil
}

// ReconciliationEngine reconciles drift between Primary Source of Truth and Target replica/cache.
type ReconciliationEngine struct {
	source ReconciliationStore
	target ReconciliationStore
}

func NewReconciliationEngine(source, target ReconciliationStore) *ReconciliationEngine {
	return &ReconciliationEngine{source: source, target: target}
}

// Reconcile runs a comparison and automatically repairs target store drift.
func (e *ReconciliationEngine) Reconcile(ctx context.Context) (*ReconciliationReport, error) {
	sourceRecords, err := e.source.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	targetRecords, err := e.target.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	report := &ReconciliationReport{}

	// 1. Check for missing or outdated records in Target
	for id, srcRec := range sourceRecords {
		tgtRec, exists := targetRecords[id]
		if !exists {
			report.MissingInTarget = append(report.MissingInTarget, id)
			_ = e.target.Upsert(ctx, srcRec)
			report.RepairedCount++
		} else if tgtRec.Version < srcRec.Version || tgtRec.Data != srcRec.Data {
			report.MismatchInTarget = append(report.MismatchInTarget, id)
			_ = e.target.Upsert(ctx, srcRec)
			report.RepairedCount++
		}
	}

	// 2. Check for orphaned records in Target that no longer exist in Source
	for id := range targetRecords {
		if _, exists := sourceRecords[id]; !exists {
			report.OrphansInTarget = append(report.OrphansInTarget, id)
			_ = e.target.Delete(ctx, id)
			report.RepairedCount++
		}
	}

	return report, nil
}
