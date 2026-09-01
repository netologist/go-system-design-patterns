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
