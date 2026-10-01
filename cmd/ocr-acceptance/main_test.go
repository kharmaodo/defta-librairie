//go:build fts5

package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"defta-librairie/internal/ocracceptance"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandRequiresEvidenceAndPreservesExistingOutput(t *testing.T) {
	root := t.TempDir()
	report := filepath.Join(root, "report.json")
	os.WriteFile(report, []byte("original"), 0600)
	if run([]string{"--output", report}) != 2 {
		t.Fatal("missing evidence accepted")
	}
	data, _ := os.ReadFile(report)
	if string(data) != "original" {
		t.Fatal("existing output changed")
	}
}
func TestCommandRealRunnersSyntheticGate(t *testing.T) {
	endpoint := os.Getenv("OCR_ACCEPTANCE_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("real local Arabic runtime integration is opt-in")
	}
	root := t.TempDir()
	database := filepath.Join(root, "catalogue.db")
	db, err := sql.Open("sqlite3", database)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT);INSERT INTO libraries VALUES('a','ACTIVE');CREATE TABLE defta(id INTEGER PRIMARY KEY,library_id TEXT,title TEXT,editeur TEXT,auteur TEXT,tags TEXT,categorie TEXT,deleted_at TEXT);CREATE VIRTUAL TABLE defta_fts USING fts5(title,editeur,auteur,tags,categorie);INSERT INTO defta VALUES(1,'a','الكتاب','','','','',NULL);INSERT INTO defta_fts(rowid,title)VALUES(1,'الكتاب');`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	id := int64(1)
	reference := "الكتاب"
	manifest := ocracceptance.Manifest{SchemaVersion: 1, Kind: "synthetic", Limits: ocracceptance.Limits{MinCases: 2, MinPositives: 1, MinNegatives: 1, MinTranscriptions: 1, MinRecall1: .8, MinRecall5: .9, MinRecall5Gain: .1, MaxNegativeCandidateRate: 0, MaxCER: .2, MaxWER: .2, MaxP95Milliseconds: 60000, MaxMemoryBytes: 1 << 30}}
	for i := 0; i < 2; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 200, 100))
		for y := 0; y < 100; y++ {
			for x := 0; x < 200; x++ {
				img.Set(x, y, color.RGBA{uint8(255 - i), uint8(255 - i), uint8(255 - i), 255})
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, img); err != nil {
			t.Fatal(err)
		}
		name := []string{"positive.png", "negative.png"}[i]
		if err := os.WriteFile(filepath.Join(root, name), buffer.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(buffer.Bytes())
		c := ocracceptance.Case{ID: strings.TrimSuffix(name, ".png"), Image: name, SHA256: hex.EncodeToString(hash[:]), LibraryID: "a", ExpectedAbsent: i == 1}
		if i == 0 {
			c.ExpectedBookID = &id
			c.ReferenceText = &reference
		}
		manifest.Cases = append(manifest.Cases, c)
	}
	data, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(root, "manifest.json")
	os.WriteFile(manifestPath, data, 0600)
	before, _ := os.ReadFile(database)
	output := filepath.Join(root, "report.json")
	args := []string{"--manifest", manifestPath, "--db", database, "--endpoint", endpoint, "--output", output}
	if code := run(args); code != 3 {
		t.Fatalf("synthetic gate should block with code 3, got %d", code)
	}
	var report ocracceptance.Report
	if _, err := readEvidence(output, &report); err != nil {
		t.Fatal(err)
	}
	if report.Baseline.Failures != 0 || report.Experimental.Failures != 0 || report.Decision != "BLOCKED" || !strings.HasPrefix(report.Records[0].Experimental.EngineVersion, "tesseract 5.") {
		t.Fatal("real runner contract failed", report.Blockers)
	}
	after, _ := os.ReadFile(database)
	if !bytes.Equal(before, after) {
		t.Fatal("catalogue changed")
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("report not private")
	}
	if code := run(args); code != 2 {
		t.Fatal("existing report overwritten")
	}
}

func TestCommandEvaluationRecomputesGateAndWritesPrivateReport(t *testing.T) {
	root := t.TempDir()
	id := int64(1)
	now := time.Now().UTC()
	report := ocracceptance.Report{SchemaVersion: 1, Kind: "real", CorpusSHA256: strings.Repeat("a", 64), CatalogueSHA256: strings.Repeat("d", 64), StartedAt: now, EndedAt: now, Limits: ocracceptance.Limits{MinCases: 2, MinPositives: 1, MinNegatives: 1, MinTranscriptions: 1, MinRecall1: .8, MinRecall5: .9, MinRecall5Gain: .1, MaxCER: .2, MaxWER: .2, MaxP95Milliseconds: 60000, MaxMemoryBytes: 1024}, Records: []ocracceptance.Record{
		{ID: "p", SHA256: strings.Repeat("b", 64), ExpectedBookID: &id, Baseline: ocracceptance.Outcome{ReferenceCharacters: 10, CharacterEdits: 2, ReferenceWords: 2, WordEdits: 1, Milliseconds: 10}, Experimental: ocracceptance.Outcome{Candidates: []int64{1}, ReferenceCharacters: 10, ReferenceWords: 2, Milliseconds: 10}},
		{ID: "n", SHA256: strings.Repeat("c", 64), ExpectedAbsent: true, Baseline: ocracceptance.Outcome{Milliseconds: 10}, Experimental: ocracceptance.Outcome{Milliseconds: 10}},
	}}
	memory := ocracceptance.MemoryEvidence{CorpusSHA256: report.CorpusSHA256, CatalogueSHA256: report.CatalogueSHA256, Method: "combined-process-peaks", BaselinePeakBytes: 100, ExperimentalPeakBytes: 100, PeakBytes: 100, ServiceImageDigest: "sha256:" + strings.Repeat("a", 64), StartedAt: now.Add(-time.Second), EndedAt: now.Add(time.Second)}
	reportPath, memoryPath, output := filepath.Join(root, "input.json"), filepath.Join(root, "memory.json"), filepath.Join(root, "review.json")
	data, _ := json.Marshal(report)
	os.WriteFile(reportPath, data, 0600)
	data, _ = json.Marshal(memory)
	os.WriteFile(memoryPath, data, 0600)
	if code := run([]string{"--evaluate", reportPath, "--memory-evidence", memoryPath, "--output", output}); code != 0 {
		t.Fatal("valid review blocked", code)
	}
	var reviewed ocracceptance.Report
	if _, err := readEvidence(output, &reviewed); err != nil || reviewed.Decision != "READY_FOR_HUMAN_REVIEW" {
		t.Fatal(err, reviewed.Decision)
	}
	info, _ := os.Stat(output)
	if info.Mode().Perm() != 0600 {
		t.Fatal("private output mode")
	}
	// Omitting the external proof blocks even a previously successful decision.
	if code := run([]string{"--evaluate", output, "--output", filepath.Join(root, "blocked.json")}); code != 3 {
		t.Fatal("stored decision trusted without evidence")
	}
}
