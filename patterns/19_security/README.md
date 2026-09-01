# 📂 Application Security & Vulnerability Defense (`patterns/19_security`)

> Application-tier security defenses against SQL injection, Server-Side Request Forgery (SSRF), Cross-Site Request Forgery (CSRF), and path traversal.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Cross-Site Request Forgery (CSRF) Protection Pattern** | [📖 Cross-Site Request Forgery (CSRF) Protection Pattern](./csrf_protection.md) | [`csrf_protection.go`](./csrf_protection.go) | [`csrf_protection_test.go`](./csrf_protection_test.go) |
| **Input Validation & Sanitization Pattern** | [📖 Input Validation & Sanitization Pattern](./input_validation.md) | [`input_validation.go`](./input_validation.go) | [`input_validation_test.go`](./input_validation_test.go) |
| **Password Hashing & Key Derivation Pattern** | [📖 Password Hashing & Key Derivation Pattern](./password_hashing.md) | [`password_hashing.go`](./password_hashing.go) | [`password_hashing_test.go`](./password_hashing_test.go) |
| **Path Traversal Defense Pattern** | [📖 Path Traversal Defense Pattern](./path_traversal.md) | [`path_traversal.go`](./path_traversal.go) | [`path_traversal_test.go`](./path_traversal_test.go) |
| **SQL Injection Defense Pattern (Parameterized Query Builder)** | [📖 SQL Injection Defense Pattern (Parameterized Query Builder)](./sql_injection_defense.md) | [`sql_injection_defense.go`](./sql_injection_defense.go) | [`sql_injection_defense_test.go`](./sql_injection_defense_test.go) |
| **Server-Side Request Forgery (SSRF) Protection Pattern** | [📖 Server-Side Request Forgery (SSRF) Protection Pattern](./ssrf_protection.md) | [`ssrf_protection.go`](./ssrf_protection.go) | [`ssrf_protection_test.go`](./ssrf_protection_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/19_security/...
go test -race ./patterns/19_security/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
