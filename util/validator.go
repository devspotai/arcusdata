package util

import (
	"fmt"
	"regexp"
)

var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func ValidateIdentifier(name string) (string, error) {
	if !identifierRegex.MatchString(name) {
		return "", fmt.Errorf("invalid SQL identifier %q", name)
	}
	return name, nil
}
