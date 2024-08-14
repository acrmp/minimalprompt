package agent

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// A SimpleFileWriter writes content to a file.
type SimpleFileWriter struct {
	logger *slog.Logger
	dir    string
}

// NewSimpleFileWriter creates a SimpleFileWriter.
// The command executes in the working directory specified with dir.
func NewSimpleFileWriter(logger *slog.Logger, dir string) *SimpleFileWriter {
	return &SimpleFileWriter{logger: logger, dir: dir}
}

// WriteFile writes the specified content to the path specified.
// Intermediate directories for path are created if they don't already exist.
// It errors if path is not local or if there is an IO error.
func (fw *SimpleFileWriter) WriteFile(path, content string) error {
	fw.logger.Info("writing file", "path", path)
	if !filepath.IsLocal(path) {
		return fmt.Errorf("path is not a local path: %q", path)
	}
	f := filepath.Join(fw.dir, path)
	if err := os.MkdirAll(filepath.Dir(f), 0700); err != nil {
		return err
	}

	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		return err
	}

	return nil
}
