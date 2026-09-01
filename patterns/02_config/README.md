# 📂 Configuration Management & Validation (`patterns/02_config`)

> Patterns for loading, strongly typing, validating, and safely sanitizing application configuration without data races during runtime.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Config Validator Pattern** | [📖 Config Validator Pattern](./config_validator.md) | [`config_validator.go`](./config_validator.go) | [`config_validator_test.go`](./config_validator_test.go) |
| **Immutable Config Pattern (Hot-Reload Safe)** | [📖 Immutable Config Pattern (Hot-Reload Safe)](./immutable_config.md) | [`immutable_config.go`](./immutable_config.go) | [`immutable_config_test.go`](./immutable_config_test.go) |
| **Secret Sanitizer Pattern** | [📖 Secret Sanitizer Pattern](./secret_sanitizer.md) | [`secret_sanitizer.go`](./secret_sanitizer.go) | [`secret_sanitizer_test.go`](./secret_sanitizer_test.go) |
| **Typed Config Pattern** | [📖 Typed Config Pattern](./typed_config.md) | [`typed_config.go`](./typed_config.go) | [`typed_config_test.go`](./typed_config_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/02_config/...
go test -race ./patterns/02_config/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
