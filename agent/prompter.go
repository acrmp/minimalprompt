package agent

import (
	"fmt"
	"io"
)

// A TerminalPrompter prompts the user for input.
type TerminalPrompter struct {
	r io.Reader
	w io.Writer
}

// NewTerminalPrompter creates a new TerminalPrompter.
func NewTerminalPrompter(r io.Reader, w io.Writer) *TerminalPrompter {
	return &TerminalPrompter{r: r, w: w}
}

// Prompt shows the prompt p to the user and returns the response when the
// reader has read to the end of the stream.
func (tp *TerminalPrompter) Prompt(p string) (string, error) {
	fmt.Fprintf(tp.w, "%s\n\nreply>", p)
	b, err := io.ReadAll(tp.r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
