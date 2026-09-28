package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid OCR input")
	ErrUnavailable  = errors.New("local OCR is unavailable")
	ErrTimeout      = errors.New("local OCR timed out")
)

// Runner keeps the worker independent from the local executable in tests.
type Runner interface {
	Run(context.Context, []byte, string) (string, error)
}

type commandFactory func(context.Context, string, ...string) *exec.Cmd

type Tesseract struct {
	binary  string
	language string
	timeout  time.Duration
	command  commandFactory
}

func NewTesseract(binary, language string, timeout time.Duration) *Tesseract {
	if strings.TrimSpace(binary) == "" {
		binary = "tesseract"
	}
	if strings.TrimSpace(language) == "" {
		language = "ara"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Tesseract{binary: binary, language: language, timeout: timeout, command: exec.CommandContext}
}

func (t *Tesseract) Run(ctx context.Context, image []byte, contentType string) (string, error) {
	if t == nil || len(image) == 0 || (contentType != "image/jpeg" && contentType != "image/png") {
		return "", ErrInvalidInput
	}
	if ctx == nil {
		return "", ErrInvalidInput
	}
	deadline, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	cmd := t.command(deadline, t.binary, "stdin", "stdout", "-l", t.language, "--psm", "6")
	cmd.Stdin = bytes.NewReader(image)
	var output bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if deadline.Err() != nil {
			return "", ErrTimeout
		}
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return strings.TrimSpace(output.String()), nil
}
