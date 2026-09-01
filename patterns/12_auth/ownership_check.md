# Resource Ownership Checks & IDOR Defense

## 1. Overview & Concept
**Insecure Direct Object Reference (IDOR)** is a critical vulnerability where an authenticated user accesses or modifies another user's private resources simply by guessing or substituting the resource identifier (e.g., `GET /invoices/9876` instead of `GET /invoices/1234`).

Resource Ownership Verification guarantees that an authenticated subject can only manipulate resources they explicitly own, unless they hold an explicit administrative bypass permission.

## 2. Production Problem & Failure Modes
Without strict ownership boundaries:
1. **Data Leakage**: An authenticated customer can view competitor invoices, medical records, or private messages by enumerating sequential IDs.
2. **Unauthorized State Mutation**: An attacker deletes or updates other tenants' accounts by submitting `DELETE /users/456`.
3. **Multi-Tenant Breaches**: Cross-tenant data contamination resulting in regulatory GDPR/HIPAA compliance violations.

## 3. Architecture & Mechanism

```text
HTTP Request (ID: "ord-99")
      |
      v
Extract Authenticated User ("usr-1")
      |
      v
Fetch Resource Record ("ord-99", Owner: "usr-2")
      |
      +---> User.ID == Resource.OwnerID? ---> Allow (200 OK)
      |
      +---> User has Bypass Role ("superadmin")? ---> Allow (200 OK)
      |
      +---> Otherwise ---> Block (403 Forbidden / ErrIDORViolation)
```

## 4. Production Hardening & Trade-offs
- **Fail-Closed by Default**: Always return `403 Forbidden` (or `404 Not Found` to prevent resource existence enumeration) if ownership fails.
- **Defense in Depth**: Perform ownership checks at both the service layer and database query filter (`WHERE id = ? AND owner_id = ?`).
- **Audit Logging**: Log every IDOR violation with user ID, target resource ID, and IP address for security operations investigation.

## 5. Code Walkthrough & Usage
See `ownership_check.go` and `ownership_check_test.go`:
- `VerifyOwnership(user, resourceOwnerID, bypassRoles...)`: Compares caller ID to resource owner and allows administrative bypass roles (e.g. `superadmin`).
