package dataintegrity_test

import (
	"sync"
	"testing"
	"time"

	dataintegrity "system-design-patterns/patterns/23_dataintegrity"
)

func TestKeyedMutexLock_SerializesSameKey(t *testing.T) {
	keyedLock := dataintegrity.NewKeyedMutexLock()

	counter := 0
	var wg sync.WaitGroup

	// 50 concurrent goroutines mutating the same key
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := keyedLock.Lock("account:100")
			defer unlock()

			temp := counter
			time.Sleep(1 * time.Millisecond)
			counter = temp + 1
		}()
	}

	wg.Wait()

	if counter != 50 {
		t.Errorf("expected serialized counter=50, got: %d", counter)
	}
}
