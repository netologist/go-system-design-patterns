package security

import (
	"errors"
	"html"
	"regexp"
	"strings"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)
)

// ValidateAndSanitizeUser performs defensive input validation and XSS HTML sanitization.
func ValidateAndSanitizeUser(username, email, bio string) (cleanBio string, err error) {
	// 1. Username
	if !usernameRegex.MatchString(username) {
		return "", errors.New("username must be 3-30 alphanumeric characters, underscores, or hyphens")
	}

	// 2. Email
	if !emailRegex.MatchString(email) {
		return "", errors.New("invalid email address format")
	}

	// 3. Bio length
	if len(bio) > 500 {
		return "", errors.New("bio exceeds maximum 500 characters limit")
	}

	// 4. Output Encoding / XSS Sanitization
	sanitizedBio := html.EscapeString(strings.TrimSpace(bio))

	return sanitizedBio, nil
}
