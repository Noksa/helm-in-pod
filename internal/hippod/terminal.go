package hippod

import (
	"os"

	"golang.org/x/term"
)

// setupTerminal puts the terminal into raw mode for interactive shell
func setupTerminal() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	return oldState, nil
}

// restoreTerminal restores the terminal to its original state
func restoreTerminal(oldState *term.State) error {
	if oldState == nil {
		return nil
	}

	fd := int(os.Stdin.Fd())
	return term.Restore(fd, oldState)
}
