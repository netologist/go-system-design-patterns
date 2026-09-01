package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Pinger defines an interface for checking connectivity to a dependency.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Dependency represents a named external service that must be validated at startup.
type Dependency struct {
	Name     string
	Critical bool
	Pinger   Pinger
	Timeout  time.Duration
}

// StartupValidator coordinates validating all dependencies before accepting traffic.
type StartupValidator struct {
	dependencies []Dependency
	totalTimeout time.Duration
}

// NewStartupValidator creates a validator with a maximum overall startup deadline.
func NewStartupValidator(totalTimeout time.Duration) *StartupValidator {
	return &StartupValidator{
		dependencies: make([]Dependency, 0),
		totalTimeout: totalTimeout,
	}
}

// AddDependency adds a dependency check to the validator.
func (v *StartupValidator) AddDependency(name string, critical bool, timeout time.Duration, pinger Pinger) {
	v.dependencies = append(v.dependencies, Dependency{
		Name:     name,
		Critical: critical,
		Pinger:   pinger,
		Timeout:  timeout,
	})
}

// ValidateAll executes all dependency checks concurrently and reports errors.
func (v *StartupValidator) ValidateAll(parentCtx context.Context) error {
	ctx, cancel := context.WithTimeout(parentCtx, v.totalTimeout)
	defer cancel()

	type result struct {
		name     string
		critical bool
		err      error
	}

	resCh := make(chan result, len(v.dependencies))
	var wg sync.WaitGroup

	for _, dep := range v.dependencies {
		wg.Add(1)
		go func(d Dependency) {
			defer wg.Done()
			depTimeout := d.Timeout
			if depTimeout <= 0 {
				depTimeout = v.totalTimeout
			}
			depCtx, depCancel := context.WithTimeout(ctx, depTimeout)
			defer depCancel()

			err := d.Pinger.Ping(depCtx)
			resCh <- result{name: d.Name, critical: d.Critical, err: err}
		}(dep)
	}

	wg.Wait()
	close(resCh)

	var criticalErrors []error
	for res := range resCh {
		if res.err != nil {
			errStr := fmt.Errorf("dependency %s check failed: %w", res.name, res.err)
			if res.critical {
				criticalErrors = append(criticalErrors, errStr)
			}
		}
	}

	if len(criticalErrors) > 0 {
		return errors.Join(criticalErrors...)
	}

	return nil
}
