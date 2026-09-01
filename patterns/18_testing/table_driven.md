# Table-Driven Testing Pattern

## 1. Overview & Concept
Table-driven testing is the idiomatic standard for writing tests in Go. It organizes test inputs, expected outputs, and test metadata into a structured slice of test cases, iterating over them using subtests (`t.Run(tc.name, ...)`).

## 2. Production Problem & Failure Modes
1. **Copy-Paste Test Proliferation**: Writing separate boilerplate test functions for every single edge case leads to code duplication, high maintenance overhead, and missed scenarios.
2. **Poor Edge-Case Coverage**: When adding test scenarios requires copying 20 lines of setup, developers avoid testing subtle boundary conditions (empty strings, unicode characters, negative values, maximum integers).
3. **Obscured Test Failures**: Without named subtests, a failure in a loop doesn't identify which specific input scenario failed.

## 3. Architecture & Mechanism

```text
Test Cases Table:
+------------------------------------+-------------------------+--------------------+
| Name                               | Input                   | Expected Output    |
+------------------------------------+-------------------------+--------------------+
| "Valid payload"                    | UserInput{Age: 25, ...} | nil                |
| "Empty username"                   | UserInput{Age: 25, ...} | ErrEmptyUsername   |
| "Invalid email format"             | UserInput{Age: 25, ...} | ErrInvalidEmail    |
| "Underage user"                    | UserInput{Age: 17, ...} | ErrUnderage        |
+------------------------------------+-------------------------+--------------------+
                  |
                  v
       for _, tc := range tests {
           t.Run(tc.name, func(t *testing.T) {
               // Execute and Assert
           })
       }
```

## 4. Production Hardening & Trade-offs
- **Isolated Subtests**: Using `t.Run()` ensures individual test case failures do not abort the entire test suite and allows running specific scenarios with `go test -run TestName/SubtestName`.
- **Parallel Subtests**: Calling `t.Parallel()` inside `t.Run()` speeds up execution across CPU cores (ensure loop variables are correctly scoped in Go < 1.22).
- **Error Assertions**: Use `errors.Is` or custom error comparator rather than fragile string comparison (`err.Error() == "expected"`).

## 5. Code Walkthrough & Usage
See `table_driven.go` and `table_driven_test.go`:
- `UserValidationService.Validate(input)` tested across comprehensive boundary conditions in a structured table.
