package security_test

import (
	"errors"
	"testing"

	security "system-design-patterns/patterns/19_security"
)

func TestValidateTargetURL_SSRFBlocked(t *testing.T) {
	// 1. AWS Cloud Metadata endpoint
	err := security.ValidateTargetURL("http://169.254.169.254/latest/meta-data/")
	if !errors.Is(err, security.ErrSSRFBlocked) {
		t.Errorf("expected cloud metadata IP blocked, got: %v", err)
	}

	// 2. Loopback 127.0.0.1
	err = security.ValidateTargetURL("http://127.0.0.1:8080/admin")
	if !errors.Is(err, security.ErrSSRFBlocked) {
		t.Errorf("expected loopback blocked, got: %v", err)
	}

	// 3. Private subnet 10.0.0.5
	err = security.ValidateTargetURL("https://10.0.0.5:5432")
	if !errors.Is(err, security.ErrSSRFBlocked) {
		t.Errorf("expected private RFC1918 IP blocked, got: %v", err)
	}

	// 4. File / Gopher scheme
	err = security.ValidateTargetURL("file:///etc/passwd")
	if !errors.Is(err, security.ErrDisallowedScheme) {
		t.Errorf("expected file:// scheme blocked, got: %v", err)
	}

	// 5. Valid Public URL
	err = security.ValidateTargetURL("https://api.github.com/users/hasan")
	if err != nil {
		t.Errorf("expected public API URL allowed, got: %v", err)
	}
}
