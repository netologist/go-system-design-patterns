# Interface at Consumer Pattern

## 1. Overview & Concept
The **Interface at Consumer** pattern (also known as *Consumer-Driven Interface Design*) is one of Go's core architectural tenets: **interfaces should be defined by the package that uses them (the consumer), not by the package that implements them (the producer)**.

Because Go uses implicit interface satisfaction (structural typing), a producer struct does not need to declare that it implements an interface. This allows consumers to define focused, minimal interfaces tailored strictly to the exact subset of methods they need to perform their business operations.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Defining monolithic interfaces on the producer/provider side (common in Java/C# ecosystems) leads to severe issues in Go codebases:

- **"Fat Interface" Bloat**: A database package defines an interface with 50 methods (`CreateUser`, `DeleteUser`, `CreateOrder`, `ListInvoices`...). A service that only needs to read a single order is forced to depend on the entire 50-method interface.
- **Testing Friction & Mock Explosion**: To unit-test a consumer that depends on a bloated producer-defined interface, test doubles must implement all 50 dummy methods, resulting in hundreds of lines of boilerplate mock code.
- **Tight Coupling Across Domains**: Changes to unrelated methods in a producer-defined interface force recompilation and refactoring across all consuming packages, even those that never use the modified methods.
- **Circular Package Dependencies**: When interfaces are defined on the provider side, consumers importing providers and providers needing consumer domain types create circular import graphs rejected by the Go compiler.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Consumer vs Producer Interface Hierarchy

```
       PRODUCER-DEFINED (Anti-pattern in Go)                     CONSUMER-DEFINED (Idiomatic Go)
       
  +--------------------------------------------+          +--------------------------------------------+
  |               PACKAGE db                   |          |               PACKAGE order                |
  | type Store interface {                     |          | // Consumer defines only what it needs!    |
  |     CreateUser(...)                        |          | type OrderStorage interface {              |
  |     DeleteUser(...)                        |          |     GetOrder(ctx, id) (*Order, error)      |
  |     GetOrder(...)                          |          |     UpdateOrderStatus(ctx, id, s) error    |
  |     UpdateOrderStatus(...)                 |          | }                                          |
  |     ... 50 more methods                    |          |                                            |
  | }                                          |          | type OrderProcessor struct {               |
  | type PostgresStore struct {}               |          |     storage OrderStorage                   |
  +--------------------------------------------+          | }                                          |
                        ^                                 +--------------------------------------------+
                        | (Forced dependency on 50 methods)                             ^
  +--------------------------------------------+                                        | (Implicit Satisfaction)
  |               PACKAGE order                |          +--------------------------------------------+
  | type OrderProcessor struct {               |          |               PACKAGE postgres             |
  |     store db.Store // Fat dependency       |          | type DBStore struct {} // No imports of order|
  | }                                          |          | func (s *DBStore) GetOrder(...)            |
  +--------------------------------------------+          | func (s *DBStore) UpdateOrderStatus(...)   |
                                                          +--------------------------------------------+
```

### Contrast: Producer-Defined vs Consumer-Defined

| Feature | Producer-Defined (Anti-pattern in Go) | Consumer-Defined (Idiomatic Go) |
| :--- | :--- | :--- |
| **Location** | Defined alongside concrete struct in provider package | Defined right next to the struct consuming the dependency |
| **Size** | Large / Monolithic (10–50 methods) | Small / Granular (1–3 methods) |
| **Coupling** | High: Consumer depends on provider package interface | Zero: Consumer only depends on its own local interface |
| **Mocking** | High friction: Must mock entire interface | Low friction: Must only mock 1–2 methods used in tests |

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Define Interfaces Next to Consumers**: Declare the interface in the same package and file where the consuming service is implemented.
- **Keep Interfaces Small (1–3 Methods)**: Follow the Single Responsibility Principle and Interface Segregation Principle (e.g., `io.Reader`, `io.Writer`). Small interfaces are easy to compose, satisfy, and mock.
- **Return Concrete Structs, Accept Interfaces**: Producer packages should return concrete pointers (`*PostgresOrderStore`) and export no broad interfaces.
- **Compose Interfaces when Needed**: If a consumer requires multiple capabilities, compose small interfaces rather than defining one large interface.

### Common Pitfalls & Anti-Patterns
- **Exporting Interfaces from Implementation Packages**: Creating a package `database` that exports `type DatabaseInterface interface { ... }` that mirrors all database methods.
- **Mocking Unused Methods**: Writing massive test mocks because the interface contains methods never invoked by the code under test.
- **Defining Interfaces Before Consumers Exist**: Creating speculative interfaces before any consumer actually needs polymorphism.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/03_di/interface_at_consumer.go`.

### Core Types & Signatures

```go
package di

import "context"

type Order struct {
	ID         string
	Amount     int64
	Status     string
	CustomerID string
}

// Consumer-defined interface: Only what OrderProcessor needs to read and write.
type OrderStorage interface {
	GetOrder(ctx context.Context, id string) (*Order, error)
	UpdateOrderStatus(ctx context.Context, id, status string) error
}

// Consumer-defined interface: Only what OrderProcessor needs to process payment.
type PaymentGateway interface {
	Charge(ctx context.Context, customerID string, amount int64) (string, error)
}

type OrderProcessor struct {
	// unexported fields: storage, payment
}

func NewOrderProcessor(storage OrderStorage, payment PaymentGateway) *OrderProcessor
func (p *OrderProcessor) ProcessOrder(ctx context.Context, orderID string) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"log"

	"patterns/03_di"
)

// Concrete implementation in an infrastructure package (satisfies interfaces implicitly)
type PostgresStore struct {
	orders map[string]*di.Order
}

func (s *PostgresStore) GetOrder(ctx context.Context, id string) (*di.Order, error) {
	return s.orders[id], nil
}

func (s *PostgresStore) UpdateOrderStatus(ctx context.Context, id, status string) error {
	if o, ok := s.orders[id]; ok {
		o.Status = status
	}
	return nil
}

// Concrete payment gateway
type StripeClient struct{}

func (s *StripeClient) Charge(ctx context.Context, customerID string, amount int64) (string, error) {
	log.Printf("Charging $%d to customer %s via Stripe", amount/100, customerID)
	return "txn_stripe_987654", nil
}

func main() {
	store := &PostgresStore{
		orders: map[string]*di.Order{
			"ord_100": {ID: "ord_100", Amount: 4999, Status: "PENDING", CustomerID: "cust_42"},
		},
	}
	payment := &StripeClient{}

	// OrderProcessor accepts anything that satisfies its consumer-defined interfaces
	processor := di.NewOrderProcessor(store, payment)

	ctx := context.Background()
	if err := processor.ProcessOrder(ctx, "ord_100"); err != nil {
		log.Fatalf("Order processing failed: %v", err)
	}

	log.Printf("Order processed successfully. Status: %s", store.orders["ord_100"].Status)
}
```
