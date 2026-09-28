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

var ErrUnavailable = errors.New("local OCR is unavailable")

// Runner is deliberately small so the worker can be tested without executing
// a binary. Input is passed through stdin; no source image is written to disk.
type Runner interface {
	Run(context.Context, []byte, string) (string, error)
}

type Tesseract struct {
	Binary  string
	Language string
	Timeout  time.Duration
}

func NewTesseract(binary, language string, timeout time.Duration) *Tesseract {
	if strings.TrimSpace(binary) == "" { binary = "tesseract" }
	if strings.TrimSpace(language) == "" { language = "ara" }
	if timeout <= 0 { timeout = 20 * time.Second }
	return &Tesseract{Binary: binary, Language: language, Timeout: timeout}
}

func (t *Tesseract) Run(ctx context.Context, image []byte, contentType string) (string, error) {
	if t == nil || len(image) == 0 || (contentType != "image/jpeg" && contentType != "image/png") {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, t.Binary, "stdin", "stdout", "-l", t.Language, "--psm", "6")
	cmd.Stdin = bytes.NewReader(image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return strings.TrimSpace(string(output)), nil
}
