package ocracceptance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"defta-librairie/internal/ocr"
	"encoding/hex"
	_ "github.com/mattn/go-sqlite3"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func limits() Limits {
	return Limits{MinCases: 2, MinPositives: 1, MinNegatives: 1, MinTranscriptions: 1, MinRecall1: .8, MinRecall5: .9, MinRecall5Gain: .1, MaxNegativeCandidateRate: 0, MaxCER: .2, MaxWER: .2, MaxP95Milliseconds: 60000, MaxMemoryBytes: 1024 * 1024 * 1024}
}
func sampleReport() Report {
	id := int64(1)
	now := time.Now().UTC()
	return Report{SchemaVersion: 1, Kind: "real", CorpusSHA256: strings.Repeat("a", 64), CatalogueSHA256: strings.Repeat("d", 64), StartedAt: now, EndedAt: now, Limits: limits(), Records: []Record{
		{ID: "positive", SHA256: strings.Repeat("b", 64), ExpectedBookID: &id, Baseline: Outcome{ReferenceCharacters: 10, CharacterEdits: 2, ReferenceWords: 2, WordEdits: 1, Milliseconds: 10}, Experimental: Outcome{Candidates: []int64{1}, ReferenceCharacters: 10, ReferenceWords: 2, Milliseconds: 20}},
		{ID: "negative", SHA256: strings.Repeat("c", 64), ExpectedAbsent: true, Baseline: Outcome{Milliseconds: 10}, Experimental: Outcome{Milliseconds: 20}},
	}}
}
func memory(report Report) *MemoryEvidence {
	return &MemoryEvidence{CorpusSHA256: report.CorpusSHA256, CatalogueSHA256: report.CatalogueSHA256, Method: "combined-process-peaks", PeakBytes: 100, BaselinePeakBytes: 100, ExperimentalPeakBytes: 100, ServiceImageDigest: "sha256:" + strings.Repeat("a", 64), StartedAt: report.StartedAt.Add(-time.Second), EndedAt: report.EndedAt.Add(time.Second)}
}
func TestAcceptanceGateNeverActivates(t *testing.T) {
	r := sampleReport()
	Evaluate(&r, memory(r))
	if r.Decision != "READY_FOR_HUMAN_REVIEW" || len(r.Blockers) != 0 {
		t.Fatal(r.Blockers)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Report, *MemoryEvidence)
	}{
		{"synthetic", func(r *Report, _ *MemoryEvidence) { r.Kind = "synthetic" }},
		{"too-small", func(r *Report, _ *MemoryEvidence) { r.Limits.MinCases = 3 }},
		{"no-gain", func(r *Report, _ *MemoryEvidence) { r.Records[0].Baseline.Candidates = []int64{1} }},
		{"new-false-candidate", func(r *Report, _ *MemoryEvidence) { r.Records[1].Experimental.Candidates = []int64{2} }},
		{"failure", func(r *Report, _ *MemoryEvidence) { r.Records[0].Experimental.Error = "OCR_FAILED" }},
		{"missing-transcript", func(r *Report, _ *MemoryEvidence) { r.Records[0].Experimental.ReferenceCharacters = 0 }},
		{"CER-regression", func(r *Report, _ *MemoryEvidence) { r.Records[0].Experimental.CharacterEdits = 4 }},
		{"WER-regression", func(r *Report, _ *MemoryEvidence) { r.Records[0].Experimental.WordEdits = 1 }},
		{"slow", func(r *Report, _ *MemoryEvidence) { r.Records[0].Experimental.Milliseconds = 60001 }},
		{"memory-budget", func(_ *Report, m *MemoryEvidence) { m.PeakBytes = 2 * 1024 * 1024 * 1024 }},
		{"unbound-memory", func(_ *Report, m *MemoryEvidence) { m.CorpusSHA256 = "other" }},
		{"wrong-period", func(r *Report, m *MemoryEvidence) { m.StartedAt = r.StartedAt.Add(time.Second) }},
		{"invalid-limits", func(r *Report, _ *MemoryEvidence) { r.Limits = Limits{} }},
		{"duplicate-images", func(r *Report, _ *MemoryEvidence) { r.Records[1].SHA256 = r.Records[0].SHA256 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := sampleReport()
			m := memory(r)
			test.mutate(&r, m)
			Evaluate(&r, m)
			if r.Decision != "BLOCKED" {
				t.Fatal("gate bypass", r)
			}
		})
	}
	r = sampleReport()
	Evaluate(&r, nil)
	if r.Decision != "BLOCKED" {
		t.Fatal("missing memory accepted")
	}
}
func TestAcceptanceMetricsUseUnicodeAndFailuresInDenominator(t *testing.T) {
	if distance([]rune("كتاب"), []rune("كتابه")) != 1 || distance(strings.Fields("كتاب جديد"), strings.Fields("كتاب قديم")) != 1 {
		t.Fatal("edit distance")
	}
	r := sampleReport()
	r.Records[0].Experimental.Error = "OCR_FAILED"
	Evaluate(&r, nil)
	if r.Experimental.Recall1 != 0 || r.Experimental.Failures != 1 || r.Experimental.Transcriptions != 0 {
		t.Fatal(r.Experimental)
	}
	if r.Experimental.P50Milliseconds != 20 || r.Experimental.P95Milliseconds != 20 {
		t.Fatal("percentiles")
	}
}
func TestManifestRequiresExplicitUniqueAnnotations(t *testing.T) {
	r := sampleReport()
	m := Manifest{SchemaVersion: 1, Kind: "real", Limits: r.Limits}
	for _, r := range r.Records {
		m.Cases = append(m.Cases, Case{ID: r.ID, Image: "image.png", SHA256: r.SHA256, LibraryID: "a", ExpectedBookID: r.ExpectedBookID, ExpectedAbsent: r.ExpectedAbsent})
	}
	if m.Validate() != nil {
		t.Fatal("valid manifest")
	}
	m.Cases[1].ExpectedAbsent = false
	if m.Validate() == nil {
		t.Fatal("missing annotation accepted")
	}
	m.Cases[1].ExpectedAbsent = true
	m.Cases[1].Image = "../private.png"
	if m.Validate() == nil {
		t.Fatal("escaping image path")
	}
	var parsed Manifest
	for _, data := range []string{`{"unknown":1}`, `{} {}`, `{"schemaVersion":NaN}`} {
		if Decode([]byte(data), &parsed) == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}

type fixedRunner struct {
	text  string
	calls int
}

func (r *fixedRunner) Run(context.Context, []byte, string) (string, error) {
	r.calls++
	return r.text, nil
}
func (r *fixedRunner) Extract(context.Context, []byte, string, string) (ocr.Result, error) {
	r.calls++
	return ocr.Result{TextRaw: r.text, EngineVersion: "tesseract 5.3.0"}, nil
}
func TestAcceptanceUsesProductionMatchingAndReadOnlyDatabase(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "copy.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT);INSERT INTO libraries VALUES('a','ACTIVE');CREATE TABLE defta(id INTEGER PRIMARY KEY,library_id TEXT,title TEXT,editeur TEXT,auteur TEXT,tags TEXT,categorie TEXT,deleted_at TEXT);CREATE VIRTUAL TABLE defta_fts USING fts5(title,editeur,auteur,tags,categorie);INSERT INTO defta VALUES(1,'a','صحيح البخاري','','','','',NULL);INSERT INTO defta_fts(rowid,title)VALUES(1,'صحيح البخاري');`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	db, err = sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var pngData bytes.Buffer
	if err = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := pngData.Bytes()
	os.WriteFile(filepath.Join(root, "sample.png"), data, 0600)
	sha := sha256.Sum256(data)
	id := int64(1)
	manifest := Manifest{SchemaVersion: 1, Kind: "synthetic", Limits: limits(), Cases: []Case{{ID: "sample", Image: "sample.png", SHA256: hex.EncodeToString(sha[:]), LibraryID: "a", ExpectedBookID: &id}}}
	text := strings.Repeat("ضجيج مجهول ", 20) + "\nصحيح البخاري"
	baseline, experimental := &fixedRunner{text: text}, &fixedRunner{text: text}
	report, err := Run(context.Background(), db, manifest, root, baseline, experimental, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if baseline.calls != 1 || experimental.calls != 1 || len(report.Records[0].Baseline.Candidates) != 0 || len(report.Records[0].Experimental.Candidates) != 1 || !report.Records[0].Experimental.TitleLineExact {
		t.Fatal(report.Records)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("catalogue modified")
	}
	if report.Decision != "BLOCKED" {
		t.Fatal("synthetic activation")
	}
	manifest.Cases[0].LibraryID = "foreign"
	if _, err = Run(context.Background(), db, manifest, root, baseline, experimental, strings.Repeat("a", 64)); err == nil {
		t.Fatal("foreign ground truth accepted")
	}
	manifest.Cases[0].LibraryID = "a"
	manifest.Cases[0].SHA256 = strings.Repeat("b", 64)
	if _, err = Run(context.Background(), db, manifest, root, baseline, experimental, strings.Repeat("a", 64)); err == nil {
		t.Fatal("wrong hash accepted")
	}
}
