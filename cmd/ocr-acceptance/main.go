//go:build fts5

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"defta-librairie/internal/ocr"
	"defta-librairie/internal/ocracceptance"
	_ "github.com/mattn/go-sqlite3"
)

func readEvidence(path string, target any) ([]byte, error) {
	// #nosec G304 -- Local operator-selected path, not an HTTP input; read-only validation/restore tool or startup database configuration.
	file, err := os.Open(path)
	if err != nil {
		return nil, ocracceptance.ErrInvalidEvidence
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2*1024*1024+1))
	if err != nil || ocracceptance.Decode(data, target) != nil {
		return nil, ocracceptance.ErrInvalidEvidence
	}
	return data, nil
}
func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	flags := flag.NewFlagSet("ocr-acceptance", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "Private annotated corpus JSON")
	dbPath := flags.String("db", "", "Consistent read-only SQLite copy")
	endpoint := flags.String("endpoint", "", "Private experimental OCR origin")
	evaluatePath := flags.String("evaluate", "", "Re-evaluate an existing private report")
	memoryPath := flags.String("memory-evidence", "", "Measured memory evidence JSON")
	output := flags.String("output", "", "New private JSON report; never overwrite")
	baselineTimeout := flags.Int("baseline-timeout-seconds", 20, "Production local runner deadline")
	experimentalTimeout := flags.Int("experimental-timeout-seconds", 60, "Production HTTP runner deadline")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *output == "" {
		return 2
	}
	var report ocracceptance.Report
	if *evaluatePath != "" {
		if *manifestPath != "" || *dbPath != "" || *endpoint != "" {
			fmt.Fprintln(os.Stderr, "INVALID_ARGUMENTS")
			return 2
		}
		if _, err := readEvidence(*evaluatePath, &report); err != nil {
			fmt.Fprintln(os.Stderr, "INVALID_REPORT")
			return 2
		}
	} else {
		if *manifestPath == "" || *dbPath == "" || *endpoint == "" || *baselineTimeout < 1 || *baselineTimeout > 180 || *experimentalTimeout < 1 || *experimentalTimeout > 180 {
			fmt.Fprintln(os.Stderr, "INVALID_ARGUMENTS")
			return 2
		}
		var manifest ocracceptance.Manifest
		data, err := readEvidence(*manifestPath, &manifest)
		if err != nil || manifest.Validate() != nil {
			fmt.Fprintln(os.Stderr, "INVALID_MANIFEST")
			return 2
		}
		experimental, err := ocr.NewHTTP(*endpoint, time.Duration(*experimentalTimeout)*time.Second)
		if err != nil {
			fmt.Fprintln(os.Stderr, "INVALID_ENDPOINT")
			return 2
		}
		database, err := filepath.Abs(*dbPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "DATABASE_UNAVAILABLE")
			return 2
		}
		// mode=ro refuses a missing file. Only the connection-local TEMP FTS writes.
		catalogueHash, err := hashCopy(database)
		if err != nil {
			fmt.Fprintln(os.Stderr, "CONSISTENT_COPY_REQUIRED")
			return 2
		}
		dsn := (&url.URL{Scheme: "file", Path: database, RawQuery: "mode=ro"}).String()
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			fmt.Fprintln(os.Stderr, "DATABASE_UNAVAILABLE")
			return 2
		}
		defer db.Close()
		if err = db.Ping(); err != nil {
			fmt.Fprintln(os.Stderr, "DATABASE_UNAVAILABLE")
			return 2
		}
		probeCtx, stopProbe := context.WithTimeout(context.Background(), 5*time.Second)
		versionData, probeErr := exec.CommandContext(probeCtx, "tesseract", "--version").Output()
		stopProbe()
		baselineVersion := strings.SplitN(strings.TrimSpace(string(versionData)), "\n", 2)[0]
		if probeErr != nil || !strings.HasPrefix(baselineVersion, "tesseract 5.") {
			fmt.Fprintln(os.Stderr, "TESSERACT_5_REQUIRED")
			return 2
		}
		digest := sha256.Sum256(data)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()
		report, err = ocracceptance.Run(ctx, db, manifest, filepath.Dir(*manifestPath), ocr.NewTesseract("tesseract", "ara", time.Duration(*baselineTimeout)*time.Second), experimental, hex.EncodeToString(digest[:]))
		afterHash, copyErr := hashCopy(database)
		if copyErr != nil || catalogueHash != afterHash {
			fmt.Fprintln(os.Stderr, "CATALOGUE_CHANGED")
			return 2
		}
		report.CatalogueSHA256 = catalogueHash
		for i := range report.Records {
			report.Records[i].Baseline.EngineVersion = baselineVersion
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "BENCHMARK_EVIDENCE_INVALID")
			return 2
		}
	}
	var memory *ocracceptance.MemoryEvidence
	if *memoryPath != "" {
		memory = &ocracceptance.MemoryEvidence{}
		if _, err := readEvidence(*memoryPath, memory); err != nil {
			fmt.Fprintln(os.Stderr, "INVALID_MEMORY_EVIDENCE")
			return 2
		}
	}
	ocracceptance.Evaluate(&report, memory)
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "OUTPUT_UNAVAILABLE")
		return 2
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(report)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		// #nosec G104 -- Cleanup operation; primary read/scan error is preserved and deferred rollback/close still applies.
		os.Remove(*output)
		fmt.Fprintln(os.Stderr, "OUTPUT_FAILED")
		return 2
	}
	fmt.Println(report.Decision)
	if report.Decision != "READY_FOR_HUMAN_REVIEW" {
		return 3
	}
	return 0
}

func hashCopy(path string) (string, error) {
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() > 0 {
		return "", ocracceptance.ErrInvalidEvidence
	}
	// #nosec G304 -- Local operator-selected path, not an HTTP input; read-only validation/restore tool or startup database configuration.
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
