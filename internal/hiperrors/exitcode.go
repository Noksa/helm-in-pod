package hiperrors

import "fmt"

const (
	// ExitCodeUnknown is returned when the exit code cannot be determined
	ExitCodeUnknown = -1
)

// ExitCodeError wraps a command's non-zero exit code so it can be
// propagated through the call chain and used as the process exit code.
type ExitCodeError struct {
	Code int32
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.Code)
}

// Is implements error comparison for ExitCodeError
func (e *ExitCodeError) Is(target error) bool {
	t, ok := target.(*ExitCodeError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
