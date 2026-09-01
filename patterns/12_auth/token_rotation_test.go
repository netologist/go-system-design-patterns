package auth_test

import (
	"errors"
	"testing"

	auth "system-design-patterns/patterns/12_auth"
)

func TestTokenRotation_ReplayAttackRevocation(t *testing.T) {
	mgr := auth.NewTokenRotationManager()

	// 1. Initial Login
	familyID, token1 := mgr.CreateFamily("user-100")

	// 2. Legitimate rotation: Token1 -> Token2
	token2, err := mgr.Rotate(familyID, token1)
	if err != nil {
		t.Fatalf("first rotation failed: %v", err)
	}

	// 3. Legitimate rotation: Token2 -> Token3
	token3, err := mgr.Rotate(familyID, token2)
	if err != nil {
		t.Fatalf("second rotation failed: %v", err)
	}

	// 4. Attacker attempts to replay stolen Token1!
	_, err = mgr.Rotate(familyID, token1)
	if !errors.Is(err, auth.ErrTokenReusedTheftDetected) {
		t.Fatalf("expected ErrTokenReusedTheftDetected on replay attack, got: %v", err)
	}

	// 5. Subsequent attempts with legitimate Token3 must now be rejected (family revoked)
	_, err = mgr.Rotate(familyID, token3)
	if !errors.Is(err, auth.ErrTokenInvalidOrRevoked) {
		t.Fatalf("expected family to be revoked after theft detection, got: %v", err)
	}
}
