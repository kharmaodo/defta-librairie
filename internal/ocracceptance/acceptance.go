// Package ocracceptance compares the actual Go runners and matching policies.
// Reports are private diagnostic artifacts, never production activation commands.
package ocracceptance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"defta-librairie/internal/covers"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
)

var ErrInvalidEvidence = errors.New("invalid acceptance evidence")

type Limits struct {
	MinCases                 int     `json:"minCases"`
	MinPositives             int     `json:"minPositives"`
	MinNegatives             int     `json:"minNegatives"`
	MinTranscriptions        int     `json:"minTranscriptions"`
	MinRecall1               float64 `json:"minRecall1"`
	MinRecall5               float64 `json:"minRecall5"`
	MinRecall5Gain           float64 `json:"minRecall5Gain"`
	MaxNegativeCandidateRate float64 `json:"maxNegativeCandidateRate"`
	MaxCER                   float64 `json:"maxCER"`
	MaxWER                   float64 `json:"maxWER"`
	MaxP95Milliseconds       float64 `json:"maxP95Milliseconds"`
	MaxMemoryBytes           int64   `json:"maxMemoryBytes"`
}

type Case struct {
	ExpectedTitle  string  `json:"-"`
	ID             string  `json:"id"`
	Image          string  `json:"image"`
	SHA256         string  `json:"sha256"`
	LibraryID      string  `json:"libraryId"`
	ExpectedBookID *int64  `json:"expectedBookId,omitempty"`
	ExpectedAbsent bool    `json:"expectedAbsent,omitempty"`
	ReferenceText  *string `json:"referenceText,omitempty"`
}

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	Limits        Limits `json:"limits"`
	Cases         []Case `json:"cases"`
}

type Outcome struct {
	TitleLineExact      bool    `json:"titleLineExact"`
	Candidates          []int64 `json:"candidates"`
	Error               string  `json:"error,omitempty"`
	Milliseconds        float64 `json:"milliseconds"`
	CharacterEdits      int     `json:"characterEdits"`
	ReferenceCharacters int     `json:"referenceCharacters"`
	WordEdits           int     `json:"wordEdits"`
	ReferenceWords      int     `json:"referenceWords"`
	EngineVersion       string  `json:"engineVersion,omitempty"`
}

type Record struct {
	ID             string  `json:"id"`
	SHA256         string  `json:"sha256"`
	ExpectedBookID *int64  `json:"expectedBookId,omitempty"`
	ExpectedAbsent bool    `json:"expectedAbsent"`
	Baseline       Outcome `json:"baseline"`
	Experimental   Outcome `json:"experimental"`
}

type Metrics struct {
	TitleLineExactRate    float64  `json:"titleLineExactRate"`
	Positives             int      `json:"positives"`
	Negatives             int      `json:"negatives"`
	Transcriptions        int      `json:"transcriptions"`
	Failures              int      `json:"failures"`
	Recall1               float64  `json:"recall1"`
	Recall5               float64  `json:"recall5"`
	NegativeCandidateRate float64  `json:"negativeCandidateRate"`
	NoCandidateRate       float64  `json:"noCandidateRate"`
	CER                   *float64 `json:"cer"`
	WER                   *float64 `json:"wer"`
	P50Milliseconds       float64  `json:"p50Milliseconds"`
	P95Milliseconds       float64  `json:"p95Milliseconds"`
}

type MemoryEvidence struct {
	BaselinePeakBytes     int64     `json:"baselinePeakBytes"`
	ExperimentalPeakBytes int64     `json:"experimentalPeakBytes"`
	CatalogueSHA256       string    `json:"catalogueSHA256"`
	CorpusSHA256          string    `json:"corpusSHA256"`
	Method                string    `json:"method"`
	PeakBytes             int64     `json:"peakBytes"`
	ServiceImageDigest    string    `json:"serviceImageDigest"`
	StartedAt             time.Time `json:"startedAt"`
	EndedAt               time.Time `json:"endedAt"`
}

type Report struct {
	CatalogueSHA256 string          `json:"catalogueSHA256"`
	SchemaVersion   int             `json:"schemaVersion"`
	Kind            string          `json:"kind"`
	CorpusSHA256    string          `json:"corpusSHA256"`
	StartedAt       time.Time       `json:"startedAt"`
	EndedAt         time.Time       `json:"endedAt"`
	Limits          Limits          `json:"limits"`
	Records         []Record        `json:"records"`
	Baseline        Metrics         `json:"baseline"`
	Experimental    Metrics         `json:"experimental"`
	Decision        string          `json:"decision"`
	Blockers        []string        `json:"blockers"`
	Memory          *MemoryEvidence `json:"memory,omitempty"`
}

func Decode(data []byte, target any) error {
	if len(data) > 2*1024*1024 {
		return ErrInvalidEvidence
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return ErrInvalidEvidence
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return ErrInvalidEvidence
	}
	return nil
}

func (m Manifest) Validate() error {
	l := m.Limits
	if m.SchemaVersion != 1 || (m.Kind != "real" && m.Kind != "synthetic") || len(m.Cases) < 1 || len(m.Cases) > 100 || l.MinCases < 1 || l.MinCases > 100 || l.MinPositives < 1 || l.MinNegatives < 1 || l.MinTranscriptions < 1 || l.MinPositives+l.MinNegatives > l.MinCases || l.MinTranscriptions > l.MinCases || l.MaxMemoryBytes < 1 || l.MaxP95Milliseconds <= 0 || math.IsInf(l.MaxP95Milliseconds, 0) || math.IsNaN(l.MaxP95Milliseconds) {
		return ErrInvalidEvidence
	}
	for _, rate := range []float64{l.MinRecall1, l.MinRecall5, l.MinRecall5Gain, l.MaxNegativeCandidateRate, l.MaxCER, l.MaxWER} {
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
			return ErrInvalidEvidence
		}
	}
	if l.MinRecall1 <= 0 || l.MinRecall5 <= 0 || l.MinRecall5Gain <= 0 {
		return ErrInvalidEvidence
	}
	ids := map[string]bool{}
	hashes := map[string]bool{}
	for _, c := range m.Cases {
		decoded, err := hex.DecodeString(c.SHA256)
		if !safeID(c.ID) || ids[c.ID] || hashes[c.SHA256] || err != nil || len(decoded) != 32 || c.SHA256 != strings.ToLower(c.SHA256) || c.LibraryID == "" || len(c.LibraryID) > 128 || c.Image == "" || filepath.IsAbs(c.Image) || filepath.Clean(c.Image) == ".." || strings.HasPrefix(filepath.Clean(c.Image), ".."+string(os.PathSeparator)) {
			return ErrInvalidEvidence
		}
		if c.ExpectedAbsent == (c.ExpectedBookID != nil) || (c.ExpectedBookID != nil && *c.ExpectedBookID <= 0) {
			return ErrInvalidEvidence
		}
		if c.ReferenceText != nil && (strings.TrimSpace(*c.ReferenceText) == "" || len(*c.ReferenceText) > 4096) {
			return ErrInvalidEvidence
		}
		ids[c.ID] = true
		hashes[c.SHA256] = true
	}
	return nil
}
func safeID(id string) bool {
	return len(id) > 0 && len(id) <= 64 && strings.IndexFunc(id, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}) < 0
}

func Run(ctx context.Context, db *sql.DB, manifest Manifest, root string, baseline ocr.Runner, experimental ocr.DetailedRunner, corpusHash string) (Report, error) {
	if manifest.Validate() != nil || db == nil || baseline == nil || experimental == nil {
		return Report{}, ErrInvalidEvidence
	}
	report := Report{SchemaVersion: 1, Kind: manifest.Kind, CorpusSHA256: corpusHash, Limits: manifest.Limits, StartedAt: time.Now().UTC()}
	repo := repositories.NewCoverImportRepository(db)
	for index, c := range manifest.Cases {
		if ctx.Err() != nil {
			return Report{}, ctx.Err()
		}
		var library string
		if err := db.QueryRowContext(ctx, `SELECT id FROM libraries WHERE id=? AND status='ACTIVE'`, c.LibraryID).Scan(&library); err != nil {
			return Report{}, ErrInvalidEvidence
		}
		if c.ExpectedBookID != nil {
			if err := db.QueryRowContext(ctx, `SELECT title FROM defta WHERE id=? AND library_id=? AND deleted_at IS NULL`, *c.ExpectedBookID, c.LibraryID).Scan(&c.ExpectedTitle); err != nil {
				return Report{}, ErrInvalidEvidence
			}
		}
		// #nosec G304 -- Local operator-selected path, not an HTTP input; read-only validation/restore tool or startup database configuration.
		file, err := os.Open(filepath.Join(root, c.Image))
		if err != nil {
			return Report{}, ErrInvalidEvidence
		}
		image, err := covers.NewValidator(10*1024*1024, 24_000_000).Validate(file, "")
		// #nosec G104 -- Cleanup operation; primary read/scan error is preserved and deferred rollback/close still applies.
		file.Close()
		if err != nil {
			return Report{}, ErrInvalidEvidence
		}
		hash := sha256.Sum256(image.Data)
		if hex.EncodeToString(hash[:]) != c.SHA256 {
			return Report{}, ErrInvalidEvidence
		}
		record := Record{ID: c.ID, SHA256: c.SHA256, ExpectedBookID: c.ExpectedBookID, ExpectedAbsent: c.ExpectedAbsent}
		// Alternate order to avoid consistently assigning warm runtime to one mode.
		baselineCall := func() {
			record.Baseline = measure(ctx, repo, c, func() (string, string, error) {
				text, err := baseline.Run(ctx, image.Data, image.ContentType)
				return text, "", err
			}, false)
		}
		experimentalCall := func() {
			record.Experimental = measure(ctx, repo, c, func() (string, string, error) {
				result, err := experimental.Extract(ctx, image.Data, image.ContentType, c.ID)
				return result.TextRaw, result.EngineVersion, err
			}, true)
		}
		if index%2 == 0 {
			baselineCall()
			experimentalCall()
		} else {
			experimentalCall()
			baselineCall()
		}
		report.Records = append(report.Records, record)
	}
	report.EndedAt = time.Now().UTC()
	Evaluate(&report, nil)
	return report, nil
}

func measure(ctx context.Context, repo *repositories.CoverImportRepository, c Case, extract func() (string, string, error), quality bool) Outcome {
	started := time.Now()
	out := Outcome{Candidates: []int64{}}
	text, version, err := extract()
	out.EngineVersion = version
	if err != nil {
		out.Error = "OCR_FAILED"
		out.Milliseconds = float64(time.Since(started).Microseconds()) / 1000
		return out
	}
	if c.ExpectedTitle != "" {
		expected := strings.ToLower(ocr.NormalizeForMatching(c.ExpectedTitle))
		for _, line := range strings.Split(text, "\n") {
			if strings.ToLower(ocr.NormalizeForMatching(line)) == expected {
				out.TitleLineExact = true
				break
			}
		}
	}
	var candidates []repositories.CoverImportCandidate
	if quality {
		candidates, err = repo.SearchQualityCoverImportCandidates(ctx, c.LibraryID, text)
	} else {
		tokens := strings.Fields(ocr.NormalizeForMatching(text))
		if len(tokens) > 12 {
			tokens = tokens[:12]
		}
		for i, word := range tokens {
			r := []rune(word)
			if len(r) > 64 {
				tokens[i] = string(r[:64])
			}
		}
		query := strings.ReplaceAll(ocr.FTSQuery(strings.Join(tokens, " ")), " AND ", " OR ")
		candidates, err = repo.SearchCoverImportCandidates(ctx, c.LibraryID, query)
	}
	if err != nil {
		out.Error = "MATCHING_FAILED"
	} else {
		for _, candidate := range candidates {
			out.Candidates = append(out.Candidates, candidate.BookID)
		}
	}
	if c.ReferenceText != nil {
		reference := strings.Join(strings.Fields(*c.ReferenceText), " ")
		actual := strings.Join(strings.Fields(text), " ")
		a, b := []rune(actual), []rune(reference)
		if len(a) > 4096 || len(b) > 4096 {
			out.Error = "METRIC_TEXT_LIMIT"
		} else {
			out.CharacterEdits = distance(a, b)
			out.ReferenceCharacters = len(b)
			out.WordEdits = distance(strings.Fields(actual), strings.Fields(reference))
			out.ReferenceWords = len(strings.Fields(reference))
		}
	}
	out.Milliseconds = float64(time.Since(started).Microseconds()) / 1000
	return out
}

func distance[T comparable](actual, reference []T) int {
	row := make([]int, len(reference)+1)
	for i := range row {
		row[i] = i
	}
	for i, a := range actual {
		diagonal := row[0]
		row[0] = i + 1
		for j, b := range reference {
			above := row[j+1]
			cost := 0
			if a != b {
				cost = 1
			}
			row[j+1] = min(row[j]+1, above+1, diagonal+cost)
			diagonal = above
		}
	}
	return row[len(reference)]
}

func summarize(records []Record, quality bool) Metrics {
	metrics := Metrics{}
	latencies := []float64{}
	titleHits := 0
	hits1, hits5, negativeCandidates, empty, characters, characterEdits, words, wordEdits := 0, 0, 0, 0, 0, 0, 0, 0
	for _, r := range records {
		out := r.Baseline
		if quality {
			out = r.Experimental
		}
		if out.Error != "" {
			metrics.Failures++
		}
		latencies = append(latencies, out.Milliseconds)
		if r.ExpectedAbsent {
			metrics.Negatives++
			if len(out.Candidates) > 0 {
				negativeCandidates++
			}
		} else {
			metrics.Positives++
			if out.Error == "" && out.TitleLineExact {
				titleHits++
			}
			if out.Error == "" && r.ExpectedBookID != nil {
				if len(out.Candidates) > 0 && out.Candidates[0] == *r.ExpectedBookID {
					hits1++
				}
				for _, id := range out.Candidates {
					if id == *r.ExpectedBookID {
						hits5++
						break
					}
				}
			}
		}
		if out.Error == "" && len(out.Candidates) == 0 {
			empty++
		}
		if out.ReferenceCharacters > 0 && out.Error == "" {
			metrics.Transcriptions++
			characters += out.ReferenceCharacters
			characterEdits += out.CharacterEdits
			words += out.ReferenceWords
			wordEdits += out.WordEdits
		}
	}
	if metrics.Positives > 0 {
		metrics.TitleLineExactRate = float64(titleHits) / float64(metrics.Positives)
		metrics.Recall1 = float64(hits1) / float64(metrics.Positives)
		metrics.Recall5 = float64(hits5) / float64(metrics.Positives)
	}
	if metrics.Negatives > 0 {
		metrics.NegativeCandidateRate = float64(negativeCandidates) / float64(metrics.Negatives)
	}
	if len(records) > 0 {
		metrics.NoCandidateRate = float64(empty) / float64(len(records))
	}
	if characters > 0 {
		value := float64(characterEdits) / float64(characters)
		metrics.CER = &value
	}
	if words > 0 {
		value := float64(wordEdits) / float64(words)
		metrics.WER = &value
	}
	sort.Float64s(latencies)
	if len(latencies) > 0 {
		metrics.P50Milliseconds = latencies[int(math.Ceil(float64(len(latencies))*.50))-1]
		metrics.P95Milliseconds = latencies[int(math.Ceil(float64(len(latencies))*.95))-1]
	}
	return metrics
}

func Evaluate(report *Report, memory *MemoryEvidence) {
	report.Baseline = summarize(report.Records, false)
	report.Experimental = summarize(report.Records, true)
	b, e, l := report.Baseline, report.Experimental, report.Limits
	report.Blockers = []string{}
	report.Memory = memory
	block := func(condition bool, code string) {
		if condition {
			report.Blockers = append(report.Blockers, code)
		}
	}
	block(!validReport(*report), "INVALID_REPORT")
	block(report.SchemaVersion != 1 || report.Kind != "real", "REAL_CORPUS_REQUIRED")
	block(len(report.Records) < l.MinCases || e.Positives < l.MinPositives || e.Negatives < l.MinNegatives, "CORPUS_TOO_SMALL")
	block(e.Failures > 0 || b.Failures > 0, "RUNTIME_FAILURE")
	block(e.Recall1 < l.MinRecall1 || e.Recall5 < l.MinRecall5 || e.Recall1 < b.Recall1 || e.Recall5-b.Recall5+1e-12 < l.MinRecall5Gain, "RECALL_GATE")
	block(e.NegativeCandidateRate > l.MaxNegativeCandidateRate || e.NegativeCandidateRate > b.NegativeCandidateRate, "FALSE_CANDIDATE_GATE")
	block(e.Transcriptions < l.MinTranscriptions || b.Transcriptions < l.MinTranscriptions || e.CER == nil || e.WER == nil || b.CER == nil || b.WER == nil, "TRANSCRIPTIONS_REQUIRED")
	if e.CER != nil && b.CER != nil {
		block(*e.CER > l.MaxCER || *e.CER > *b.CER, "CER_GATE")
	}
	if e.WER != nil && b.WER != nil {
		block(*e.WER > l.MaxWER || *e.WER > *b.WER, "WER_GATE")
	}
	block(e.P95Milliseconds > l.MaxP95Milliseconds, "LATENCY_GATE")
	imageDigestValid := false
	if memory != nil && strings.HasPrefix(memory.ServiceImageDigest, "sha256:") {
		digest, err := hex.DecodeString(strings.TrimPrefix(memory.ServiceImageDigest, "sha256:"))
		imageDigestValid = err == nil && len(digest) == 32
	}
	validMemory := imageDigestValid && memory != nil && !memory.StartedAt.IsZero() && !memory.EndedAt.IsZero() && memory.CorpusSHA256 == report.CorpusSHA256 && memory.CatalogueSHA256 == report.CatalogueSHA256 && memory.BaselinePeakBytes > 0 && memory.ExperimentalPeakBytes > 0 && memory.PeakBytes == max(memory.BaselinePeakBytes, memory.ExperimentalPeakBytes) && memory.Method == "combined-process-peaks" && strings.HasPrefix(memory.ServiceImageDigest, "sha256:") && len(memory.ServiceImageDigest) == 71 && !memory.StartedAt.After(report.StartedAt) && !memory.EndedAt.Before(report.EndedAt)
	block(!validMemory, "MEMORY_EVIDENCE_REQUIRED")
	if validMemory {
		block(memory.PeakBytes > l.MaxMemoryBytes, "MEMORY_GATE")
	}
	report.Decision = "BLOCKED"
	if len(report.Blockers) == 0 {
		report.Decision = "READY_FOR_HUMAN_REVIEW"
	}
}

func validReport(report Report) bool {
	catalogue, err := hex.DecodeString(report.CatalogueSHA256)
	if err != nil || len(catalogue) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(report.CorpusSHA256)
	if err != nil || len(decoded) != 32 || report.StartedAt.IsZero() || report.EndedAt.Before(report.StartedAt) {
		return false
	}
	manifest := Manifest{SchemaVersion: report.SchemaVersion, Kind: report.Kind, Limits: report.Limits}
	for _, r := range report.Records {
		manifest.Cases = append(manifest.Cases, Case{ID: r.ID, Image: "sample", SHA256: r.SHA256, LibraryID: "scoped", ExpectedBookID: r.ExpectedBookID, ExpectedAbsent: r.ExpectedAbsent})
		for _, o := range []Outcome{r.Baseline, r.Experimental} {
			if math.IsNaN(o.Milliseconds) || math.IsInf(o.Milliseconds, 0) || o.Milliseconds < 0 || len(o.Candidates) > 5 || o.CharacterEdits < 0 || o.ReferenceCharacters < 0 || o.WordEdits < 0 || o.ReferenceWords < 0 {
				return false
			}
			seen := map[int64]bool{}
			for _, id := range o.Candidates {
				if id <= 0 || seen[id] {
					return false
				}
				seen[id] = true
			}
		}
	}
	return manifest.Validate() == nil
}
