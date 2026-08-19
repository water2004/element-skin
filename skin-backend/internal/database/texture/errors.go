package texture

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrUserIDRequired = errors.New("user ID required")
)
