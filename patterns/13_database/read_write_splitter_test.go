package database_test

import (
	"context"
	"testing"
	"time"

	database "system-design-patterns/patterns/13_database"
)
type endpointMockDB struct {
	ep database.DBEndpointType
}

func (m *endpointMockDB) Exec(ctx context.Context, query string) error { return nil }
func (m *endpointMockDB) Query(ctx context.Context, query string) (string, error) {
	return string(m.ep), nil
}

func TestReadWriteRouter_ReplicaLagAwareness(t *testing.T) {
	primary := &endpointMockDB{ep: "PRIMARY"}
	replica := &endpointMockDB{ep: "REPLICA"}

	router := database.NewReadWriteRouter(primary, replica)
	ctx := context.Background()

	// 1. Normal read -> Goes to REPLICA
	ep, _, err := router.Read(ctx, "SELECT * FROM users")
	if err != nil || ep != database.EndpointReplica {
		t.Errorf("expected normal read to go to REPLICA, got: %s", ep)
	}

	// 2. Perform write and attach recent write session context (5 seconds window)
	_ = router.Write(ctx, "INSERT INTO users...")
	sessionCtx := database.WithRecentWrite(ctx, 5*time.Second)

	// 3. Read within session window -> Routes to PRIMARY to prevent stale reads
	ep, _, err = router.Read(sessionCtx, "SELECT * FROM users WHERE id = 123")
	if err != nil || ep != database.EndpointPrimary {
		t.Errorf("expected read-your-own-writes to go to PRIMARY, got: %s", ep)
	}
}
