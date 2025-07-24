package verapack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// lineCounterWriter is a wrapper for an io.Writer, that counts the number of new line tokens that are being written.
type lineCounterWriter struct {
	writer    io.Writer
	startLine int
	endLine   int
}

func (c *lineCounterWriter) Write(p []byte) (n int, err error) {
	if i := bytes.Count(p, []byte{'\n'}); i >= 0 {
		c.endLine += i
	}

	return c.writer.Write(p)
}

func (c *lineCounterWriter) GetEndLine() int {
	return c.endLine
}

func newLineCounterWriter(startLine int, writer io.Writer) *lineCounterWriter {
	if writer == nil {
		panic("writer is nil")
	}

	return &lineCounterWriter{
		writer:    writer,
		startLine: startLine,
	}
}

// initializeLogWriter opens a log file in the user's temp directory and initializes the [lineCounterWriter],
// with the file as the [io.Writer] input.
func initializeLogWriter(applicationName string) (*lineCounterWriter, func() error, error) {
	path := filepath.Join(os.TempDir(), "verapack", "logs")
	err := os.MkdirAll(path, 0600)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Create(filepath.Join(path, fmt.Sprintf("%s_latest.log", strings.ReplaceAll(applicationName, " ", "_"))))
	if err != nil {
		return nil, nil, err
	}

	return newLineCounterWriter(0, file), file.Close, nil
}
