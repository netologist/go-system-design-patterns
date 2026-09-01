# Path Traversal Defense Pattern

## 1. Overview & Concept
The **Path Traversal Defense Pattern** (also known as Directory Traversal or Dot-Dot-Slash defense) guarantees that file system operations triggered by user-supplied subpaths remain strictly confined within a designated root base directory. When web applications serve static assets, process uploaded files, download attachments, or access dynamic file templates based on URL parameters, attackers attempt to break out of the directory sandbox using relative traversal sequences (`../`, `..\`), absolute paths, null-byte injections, or symlink dereferencing.

This pattern enforces strict sanitization, null-byte checks, absolute path canonicalization (`filepath.Abs`), and post-join boundary containment verification (`filepath.Rel`) before any file descriptor is opened.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
Path traversal vulnerabilities (CWE-22) allow unauthenticated remote attackers to read sensitive system configuration files, source code, cryptographic credentials, or overwrite executable files.

### Failure Scenarios Without This Pattern
- **Sensitive File Exfiltration:** An endpoint like `GET /api/files?name=../../../../etc/passwd` or `../../../../etc/shadow` exposes system user accounts or Kubernetes service account tokens (`/var/run/secrets/kubernetes.io/serviceaccount/token`).
- **Cloud Secret Leakage:** Attackers retrieve `.env` files, database credentials, TLS private keys, and application source code by traversing to the parent container directories.
- **Arbitrary File Overwrite & Remote Code Execution:** An unvalidated file upload handler allows `../../etc/cron.d/malicious_job` or `../../root/.ssh/authorized_keys`, granting immediate root shell access to the host or container.
- **Null-Byte Injection & Extension Bypass:** Suffixes like `invoice.pdf%00.png` can confuse downstream C libraries or legacy wrappers into bypassing file extension validation.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `SafeJoinPath` function acts as a cryptographic sandbox validator for file system paths:

```
        [ Untrusted User Subpath: "../../etc/passwd" ]
                              │
                              ▼
               ┌──────────────────────────────┐
               │        SafeJoinPath          │
               └──────────────┬───────────────┘
                              │
       1. Null-Byte Check: contains '\x00'?
                              │
       2. Base Directory Canonicalization: filepath.Abs(baseDir)
                              │
       3. Clean Subpath: filepath.Clean(filepath.FromSlash(subpath))
          - Reject if starts with "..", "/", or is Abs
                              │
       4. Joined Absolute Path: filepath.Join(absBase, cleanedSub)
                              │
       5. Containment Check: filepath.Rel(absBase, joined)
                              │
               ┌──────────────┴──────────────┐
               │ Does Rel start with ".."?   │
               └──────────────┬──────────────┘
                     │                 │
                    [Yes]             [No]
                     │                 │
                     ▼                 ▼
          [ErrPathTraversalDetected] [Return Safe Absolute Path]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Use `filepath.Rel` Verification:** Simply using `filepath.Clean` is insufficient because clean paths can still point outside when joined to a base directory. Always verify `filepath.Rel(baseDir, targetPath)` does not begin with `..`.
- **Enforce OS-Agnostic Path Separators:** Use `filepath.FromSlash` to normalize Windows backslashes (`\`) and Unix forward slashes (`/`) consistently across platforms.
- **Check for Symlink Escapes:** If user-created symlinks are permitted on the filesystem, use `filepath.EvalSymlinks` to verify that resolved symlink targets also stay within the base sandbox.
- **Avoid Serving Directly from Root Directories:** Never configure the base directory to root `/` or `/var` where parent traversal escapes to system internals.

### Pitfalls to Avoid
- **Naive String Replacement:** Replacing `../` with `""` once is vulnerable to nested payloads like `....//` or `..././` which reduce back to `../` after single-pass replacement.
- **Relying Only on URL Decoding:** Decoding input once may fail against double-encoded attacks (`%252e%252e%252f`).
- **Ignoring Null Bytes:** Null bytes (`\x00`) can truncate strings in underlying OS syscalls, turning `image.png\x00.exe` into `image.png`.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/path_traversal.go`
```go
package security

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversalDetected = errors.New("security violation: path traversal outside base directory detected")
	ErrInvalidNullByte       = errors.New("invalid path: null byte detected")
)

// SafeJoinPath resolves user-supplied subpath inside baseDir, preventing directory traversal attacks.
func SafeJoinPath(baseDir, userSubpath string) (string, error) {
	// 1. Reject null bytes
	if strings.ContainsRune(userSubpath, '\x00') {
		return "", ErrInvalidNullByte
	}

	// 2. Clean base directory
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	// 3. Reject leading slash, absolute paths, or leading .. traversal
	cleanedSub := filepath.Clean(filepath.FromSlash(userSubpath))
	if strings.HasPrefix(cleanedSub, "..") || filepath.IsAbs(userSubpath) || strings.HasPrefix(userSubpath, "/") {
		return "", fmt.Errorf("%w: attempted path %s", ErrPathTraversalDetected, userSubpath)
	}

	joined := filepath.Join(absBase, cleanedSub)
	rel, err := filepath.Rel(absBase, joined)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%w: attempted path %s", ErrPathTraversalDetected, userSubpath)
	}

	return joined, nil
}
```

### Production File Server Handler Example
```go
func HandleFileDownload(storageRoot string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fileName := r.URL.Query().Get("file")
        if fileName == "" {
            http.Error(w, "missing file parameter", http.StatusBadRequest)
            return
        }

        // Enforce path traversal sandbox
        safePath, err := security.SafeJoinPath(storageRoot, fileName)
        if err != nil {
            http.Error(w, "access denied: invalid file path", http.StatusForbidden)
            return
        }

        http.ServeFile(w, r, safePath)
    }
}
```
