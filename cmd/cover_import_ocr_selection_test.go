//go:build fts5

package main

import (
	"context"
	"defta-librairie/internal/config"
	"defta-librairie/internal/ocr"
	"testing"
	"time"
)

func TestOCRFeatureFlagSelectsExactlyOneEngine(t *testing.T) {
	cfg := &config.Config{OCREngine: "tesseract", OCRLanguage: "ara", OCRTimeout: time.Second, OCRExperimentalEndpoint: "invalid", OCRExperimentalTimeout: time.Second}
	runner, err := coverImportOCRRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runner.(*ocr.Tesseract); !ok {
		t.Fatal("off must use the existing local runner")
	}
	cfg.OCRExperimentalEnabled = true
	if _, err = coverImportOCRRunner(cfg); err == nil {
		t.Fatal("on must not fall back on invalid configuration")
	}
	cfg.OCRExperimentalEndpoint = "http://127.0.0.1:1"
	runner, err = coverImportOCRRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runner.(*ocr.HTTP); !ok {
		t.Fatal("on must use HTTP")
	}
	if _, err = runner.Run(context.Background(), nil, "image/png"); err == nil {
		t.Fatal("invalid input accepted")
	}
}
