package config

import "fmt"

// CORSError builds a config validation error for CORS settings.
func CORSError(msg string) error {
	return fmt.Errorf("%s: %s", ErrCodeConfigInvalid, msg)
}
