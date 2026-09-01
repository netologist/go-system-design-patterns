package distributed_test

import (
	"context"
	"errors"
	"testing"

	distributed "system-design-patterns/patterns/16_distributed"
)

func TestSagaOrchestrator_Success(t *testing.T) {
	var trace []string

	saga := distributed.NewSagaOrchestrator(
		distributed.SagaStep{
			Name:   "BookFlight",
			Action: func(c context.Context) error { trace = append(trace, "FLIGHT_BOOKED"); return nil },
			Compensate: func(c context.Context) error { trace = append(trace, "FLIGHT_CANCELLED"); return nil },
		},
		distributed.SagaStep{
			Name:   "BookHotel",
			Action: func(c context.Context) error { trace = append(trace, "HOTEL_BOOKED"); return nil },
			Compensate: func(c context.Context) error { trace = append(trace, "HOTEL_CANCELLED"); return nil },
		},
	)

	err := saga.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected saga to succeed, got: %v", err)
	}

	if len(trace) != 2 || trace[0] != "FLIGHT_BOOKED" || trace[1] != "HOTEL_BOOKED" {
		t.Errorf("unexpected execution trace: %v", trace)
	}
}

func TestSagaOrchestrator_CompensationOnFailure(t *testing.T) {
	var trace []string

	saga := distributed.NewSagaOrchestrator(
		distributed.SagaStep{
			Name:   "BookFlight",
			Action: func(c context.Context) error { trace = append(trace, "FLIGHT_BOOKED"); return nil },
			Compensate: func(c context.Context) error { trace = append(trace, "FLIGHT_CANCELLED"); return nil },
		},
		distributed.SagaStep{
			Name:   "BookHotel",
			Action: func(c context.Context) error { trace = append(trace, "HOTEL_BOOKED"); return nil },
			Compensate: func(c context.Context) error { trace = append(trace, "HOTEL_CANCELLED"); return nil },
		},
		distributed.SagaStep{
			Name:   "BookRentalCar",
			Action: func(c context.Context) error { return errors.New("no cars available") }, // Fails here!
			Compensate: func(c context.Context) error { trace = append(trace, "CAR_CANCELLED"); return nil },
		},
	)

	err := saga.Execute(context.Background())
	if err == nil {
		t.Fatal("expected saga failure, got nil")
	}

	// Hotel and Flight must be compensated in reverse order: HOTEL_CANCELLED then FLIGHT_CANCELLED
	expected := []string{"FLIGHT_BOOKED", "HOTEL_BOOKED", "HOTEL_CANCELLED", "FLIGHT_CANCELLED"}
	if len(trace) != len(expected) {
		t.Fatalf("expected trace %v, got %v", expected, trace)
	}

	for i, step := range expected {
		if trace[i] != step {
			t.Errorf("at index %d: expected %s, got %s", i, step, trace[i])
		}
	}
}
