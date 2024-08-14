package agent

import (
	"log/slog"
	"os/exec"
)

// A BashExecutor executes bash commands
type BashExecutor struct {
	logger *slog.Logger
	dir    string
}

// NewBashExecutor creates a BashExecutor.
// The command executes in the working directory specified with dir.
func NewBashExecutor(logger *slog.Logger, dir string) *BashExecutor {
	return &BashExecutor{logger: logger, dir: dir}
}

// Execute runs the bash command represented by cmd and returns the combined
// output of stdout and sterr as a string as well as any error.
func (b *BashExecutor) Execute(cmd string) (string, error) {
	b.logger.Info("executing command", "command", cmd)
	c := exec.Command("/usr/bin/bash", "-c", cmd)
	c.Dir = b.dir

	o, err := c.CombinedOutput()
	return string(o), err
}
