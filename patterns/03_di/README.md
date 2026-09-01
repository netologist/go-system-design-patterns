# 📂 Dependency Injection & Clean Interfaces (`patterns/03_di`)

> Idiomatic Go dependency injection techniques, consumer-driven interfaces, functional options, and test doubles (fakes, stubs, mocks).

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Constructor Injection Pattern** | [📖 Constructor Injection Pattern](./constructor_injection.md) | [`constructor_injection.go`](./constructor_injection.go) | [`constructor_injection_test.go`](./constructor_injection_test.go) |
| **Functional Options Pattern** | [📖 Functional Options Pattern](./functional_options.md) | [`functional_options.go`](./functional_options.go) | [`functional_options_test.go`](./functional_options_test.go) |
| **Interface at Consumer Pattern** | [📖 Interface at Consumer Pattern](./interface_at_consumer.md) | [`interface_at_consumer.go`](./interface_at_consumer.go) | [`interface_at_consumer_test.go`](./interface_at_consumer_test.go) |
| **Test Doubles Pattern (Fakes, Stubs, Spies & Mocks)** | [📖 Test Doubles Pattern (Fakes, Stubs, Spies & Mocks)](./test_doubles.md) | [`test_doubles.go`](./test_doubles.go) | [`test_doubles_test.go`](./test_doubles_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/03_di/...
go test -race ./patterns/03_di/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
