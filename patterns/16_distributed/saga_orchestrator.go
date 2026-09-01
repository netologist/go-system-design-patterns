package distributed

import (
	"context"
	"fmt"
)

// SagaStep defines a single distributed transaction step with forward action and backward compensation.
type SagaStep struct {
	Name         string
	Action       func(ctx context.Context) error
	Compensate   func(ctx context.Context) error
}

// SagaOrchestrator executes multi-service transactions with automatic backward compensation on failure.
type SagaOrchestrator struct {
	steps []SagaStep
}

func NewSagaOrchestrator(steps ...SagaStep) *SagaOrchestrator {
	return &SagaOrchestrator{
		steps: steps,
	}
}

// Execute runs all saga steps sequentially. If any step fails, compensations for previously completed steps are executed in reverse order.
func (s *SagaOrchestrator) Execute(ctx context.Context) error {
	var completedSteps []SagaStep

	for _, step := range s.steps {
		if err := step.Action(ctx); err != nil {
			// Step failed -> Trigger compensation rollbacks in reverse order
			compErr := s.compensate(ctx, completedSteps)
			if compErr != nil {
				return fmt.Errorf("saga step '%s' failed (%w); compensation error: %v", step.Name, err, compErr)
			}
			return fmt.Errorf("saga step '%s' failed and successfully compensated: %w", step.Name, err)
		}
		completedSteps = append(completedSteps, step)
	}

	return nil
}

func (s *SagaOrchestrator) compensate(ctx context.Context, steps []SagaStep) error {
	var compErrors []error
	// Execute in reverse order (LIFO)
	for i := len(steps) - 1; i >= 0; i-- {
		step := steps[i]
		if step.Compensate != nil {
			if err := step.Compensate(ctx); err != nil {
				compErrors = append(compErrors, fmt.Errorf("compensation '%s' failed: %w", step.Name, err))
			}
		}
	}

	if len(compErrors) > 0 {
		return fmt.Errorf("compensation failures: %v", compErrors)
	}
	return nil
}
