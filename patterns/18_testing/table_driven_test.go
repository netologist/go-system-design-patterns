package testingpattern_test

import (
	"errors"
	"testing"

	testingpattern "system-design-patterns/patterns/18_testing"
)

func TestUserValidationService_TableDriven(t *testing.T) {
	svc := &testingpattern.UserValidationService{}

	tests := []struct {
		name        string
		input       testingpattern.UserInput
		expectedErr error
	}{
		{
			name: "Valid user passes validation",
			input: testingpattern.UserInput{
				Username: "hasan_ozgan",
				Email:    "hasan@example.com",
				Age:      30,
			},
			expectedErr: nil,
		},
		{
			name: "Empty username fails",
			input: testingpattern.UserInput{
				Username: "   ",
				Email:    "hasan@example.com",
				Age:      25,
			},
			expectedErr: testingpattern.ErrEmptyUsername,
		},
		{
			name: "Invalid email without domain fails",
			input: testingpattern.UserInput{
				Username: "hasan",
				Email:    "invalid-email-string",
				Age:      25,
			},
			expectedErr: testingpattern.ErrInvalidEmail,
		},
		{
			name: "Underage user fails",
			input: testingpattern.UserInput{
				Username: "young_dev",
				Email:    "young@example.com",
				Age:      17,
			},
			expectedErr: testingpattern.ErrUnderage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Validate(tc.input)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got: %v", tc.expectedErr, err)
			}
		})
	}
}
