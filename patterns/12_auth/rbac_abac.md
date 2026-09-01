# Role-Based (RBAC) & Attribute-Based (ABAC) Access Control

## 1. Overview & Concept
Authorization answers: *"Does this authenticated principal have permission to perform this specific action on this target resource in the current context?"*

- **Role-Based Access Control (RBAC)**: Maps permissions to abstract roles (`admin`, `editor`, `viewer`). Users inherit permissions assigned to their roles.
- **Attribute-Based Access Control (ABAC)**: Evaluates dynamic context attributes (e.g., resource ownership, department clearance, time of day, confidentiality flags) at runtime.

## 2. Production Problem & Failure Modes
1. **RBAC Role Explosion**: Attempting to express fine-grained context (e.g., *"only editors in the London office during business hours can edit invoices over $10k"*) using pure RBAC leads to thousands of specialized roles (`London_Day_Invoice_Editor_Role`).
2. **Hardcoded Authorization Logic**: Scattering `if user.Role == "admin" || user.ID == doc.OwnerID` across handlers creates security blind spots and makes compliance audits impossible.
3. **Privilege Creep**: Static roles grant broad permissions without verifying environmental or resource attributes, allowing lateral privilege escalation.

## 3. Architecture & Mechanism

```text
Request ---> Authn Context ---> RBAC Evaluation ---> ABAC Dynamic Rules ---> Authorized (200)
                                      |                     |
                                   No Role?           Attr Mismatch?
                                      v                     v
                                  403 Forbidden         403 Forbidden
```

## 4. Production Hardening & Trade-offs
- **Fail-Closed Default**: If an attribute is missing or ambiguous, access must always be denied.
- **Layered Authorization**: Use RBAC as a fast, high-level filter (coarse-grained) followed by ABAC for contextual constraints (fine-grained).
- **Performance**: Cache role-to-permission mappings in memory; evaluate lightweight attributes without extra database lookups whenever possible.

## 5. Code Walkthrough & Usage
See `rbac_abac.go` and `rbac_abac_test.go`:
- `RBACRegistry`: Manages `RoleAdmin`, `RoleEditor`, `RoleViewer` permission sets.
- `ABACAuthorizer`: Dynamically checks `IsConfidential`, `ResourceOwnerID`, and `Department` clearance.
