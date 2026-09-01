package di_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	di "system-design-patterns/patterns/03_di"
)

func TestFakeAccountRepository_ConcurrencyAndPersistence(t *testing.T) {
	fake := di.NewFakeAccountRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = fake.Save(ctx, &di.Account{
				ID:      "acc-1",
				Balance: int64(idx * 100),
			})
		}(i)
	}
	wg.Wait()

	acc, err := fake.Get(ctx, "acc-1")
	if err != nil {
		t.Fatalf("expected account to exist: %v", err)
	}
	if acc.ID != "acc-1" {
		t.Errorf("expected ID acc-1, got: %s", acc.ID)
	}
}

func TestStubAccountRepository(t *testing.T) {
	cannedErr := errors.New("database locked")
	stub := &di.StubAccountRepository{
		CannedError: cannedErr,
	}

	_, err := stub.Get(context.Background(), "any-id")
	if !errors.Is(err, cannedErr) {
		t.Errorf("expected canned error, got: %v", err)
	}
}

func TestSpyAccountRepository(t *testing.T) {
	spy := &di.SpyAccountRepository{}
	ctx := context.Background()

	_, _ = spy.Get(ctx, "acc-99")
	_ = spy.Save(ctx, &di.Account{ID: "acc-99", Balance: 500})

	if len(spy.GetCalls) != 1 || spy.GetCalls[0] != "acc-99" {
		t.Errorf("spy did not record Get call correctly: %v", spy.GetCalls)
	}

	if len(spy.SaveCalls) != 1 || spy.SaveCalls[0].Balance != 500 {
		t.Errorf("spy did not record Save call correctly: %v", spy.SaveCalls)
	}
}
