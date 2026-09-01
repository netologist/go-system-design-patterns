# 📂 Authentication, JWT, Token Rotation & RBAC (`patterns/12_auth`)

> Enterprise identity and access management patterns including stateless JWT validation, cryptographic token rotation, and RBAC/ABAC authorization.

---

## 📋 Included Patterns

| Pattern | Architectural Specification | Go Source Code | Unit & Concurrency Tests |
| :--- | :--- | :--- | :--- |
| **Authentication & Fail-Closed Authorization Core** | [📖 Authentication & Fail-Closed Authorization Core](./auth_core.md) | [`auth_core.go`](./auth_core.go) | [`auth_core_test.go`](./auth_core_test.go) |
| **Cryptographic JWT Validation & Signing** | [📖 Cryptographic JWT Validation & Signing](./jwt_validator.md) | [`jwt_validator.go`](./jwt_validator.go) | [`jwt_validator_test.go`](./jwt_validator_test.go) |
| **Resource Ownership Checks & IDOR Defense** | [📖 Resource Ownership Checks & IDOR Defense](./ownership_check.md) | [`ownership_check.go`](./ownership_check.go) | [`ownership_check_test.go`](./ownership_check_test.go) |
| **Role-Based (RBAC) & Attribute-Based (ABAC) Access Control** | [📖 Role-Based (RBAC) & Attribute-Based (ABAC) Access Control](./rbac_abac.md) | [`rbac_abac.go`](./rbac_abac.go) | [`rbac_abac_test.go`](./rbac_abac_test.go) |
| **Refresh Token Rotation & Breach Detection (Token Families)** | [📖 Refresh Token Rotation & Breach Detection (Token Families)](./token_rotation.md) | [`token_rotation.go`](./token_rotation.go) | [`token_rotation_test.go`](./token_rotation_test.go) |

---

## 🧪 Running Tests for This Package
```bash
go test -v ./patterns/12_auth/...
go test -race ./patterns/12_auth/...
```

---

## 🌐 Digital Garden Reference
Interactive explanations, architecture diagrams, and deep dives are published on **[netologist.org](https://netologist.org)**.

[⬅️ Back to Main Repository README](../../README.md)
