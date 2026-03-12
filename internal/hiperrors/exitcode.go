package hiperrors

import "fmt"

// ExitCodeError wraps a command's non-zero exit code so it can be
// propagated through the call chain and used as the process exit code.
type ExitCodeError struct {
	Code int32
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("command exited with code %d", e.Code)
}
