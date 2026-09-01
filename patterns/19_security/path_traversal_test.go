package security_test

import (
	"errors"
	"strings"
	"testing"

	security "system-design-patterns/patterns/19_security"
)

func TestSafeJoinPath(t *testing.T) {
	baseDir := "/var/app/uploads"

	// 1. Valid safe file path
	safeTarget, err := security.SafeJoinPath(baseDir, "images/photo.png")
	if err != nil {
		t.Fatalf("expected safe path allowed, got error: %v", err)
	}
	if !strings.HasSuffix(safeTarget, "/var/app/uploads/images/photo.png") {
		t.Errorf("unexpected target path: %s", safeTarget)
	}

	// 2. Directory traversal attempt: ../../etc/passwd
	_, err = security.SafeJoinPath(baseDir, "../../../../etc/passwd")
	if err == nil {
		t.Fatal("expected path traversal to be detected and blocked, got nil")
	}

	// 3. Null byte injection
	_, err = security.SafeJoinPath(baseDir, "photo.png\x00.exe")
	if !errors.Is(err, security.ErrInvalidNullByte) {
		t.Errorf("expected ErrInvalidNullByte, got: %v", err)
	}
}
