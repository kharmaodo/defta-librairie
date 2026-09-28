package ocr

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTesseractRejectsInvalidInput(t *testing.T) {
	runner := NewTesseract("tesseract", "ara", time.Second)
	_, err := runner.Run(context.Background(), nil, "image/jpeg")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewTesseractAppliesSafeDefaults(t *testing.T) {
	runner := NewTesseract("", "", 0)
	if runner.binary != "tesseract" || runner.language != "ara" || runner.timeout != 20*time.Second {
		t.Fatalf("unexpected defaults: %#v", runner)
	}
}
