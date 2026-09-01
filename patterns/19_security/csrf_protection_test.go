package security_test

import (
	"errors"
	"testing"
	"time"

	security "system-design-patterns/patterns/19_security"
)

func TestCSRFManager_ValidAndTampered(t *testing.T) {
	mgr := security.NewCSRFManager("secret-key-1234", 1*time.Hour)

	sessionID := "sess-user-99"

	// 1. Generate & Validate
	token, err := mgr.GenerateToken(sessionID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if err := mgr.ValidateToken(sessionID, token); err != nil {
		t.Fatalf("expected valid token to pass, got: %v", err)
	}

	// 2. Mismatched session ID -> Rejected
	if err := mgr.ValidateToken("different-session", token); !errors.Is(err, security.ErrCSRFInvalid) {
		t.Errorf("expected ErrCSRFInvalid for wrong session ID, got: %v", err)
	}

	// 3. Tampered payload
	tampered := "tampered-payload" + token[10:]
	if err := mgr.ValidateToken(sessionID, tampered); !errors.Is(err, security.ErrCSRFInvalid) {
		t.Errorf("expected ErrCSRFInvalid for tampered token, got: %v", err)
	}
}

func TestCSRFManager_Expired(t *testing.T) {
	// Negative TTL -> immediately expired
	mgr := security.NewCSRFManager("secret-key-1234", -1*time.Second)

	token, _ := mgr.GenerateToken("sess-1")
	err := mgr.ValidateToken("sess-1", token)

	if !errors.Is(err, security.ErrCSRFExpired) {
		t.Errorf("expected ErrCSRFExpired, got: %v", err)
	}
}
