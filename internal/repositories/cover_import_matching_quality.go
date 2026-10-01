package repositories

import (
	"context"
	"defta-librairie/internal/ocr"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const MaxMatchingCatalogueBooks = 10000
const MaxMatchingDictionaryWords = 20000

var matchingTableID atomic.Uint64
var ErrMatchingCatalogueLimit = errors.New("matching catalogue exceeds local budget")

type matchingCatalogueBook struct {
	id     int64
	fields [5]string
}

// SearchQualityCoverImportCandidates builds a connection-local, normalized FTS
// snapshot of this library only. BM25 statistics never include another library.
// No private text is persisted in a new table or exposed in errors/logs.
func (r *CoverImportRepository) SearchQualityCoverImportCandidates(parent context.Context, libraryID, text string) ([]CoverImportCandidate, error) {
	if libraryID == "" || len(text) > 1024*1024 {
		return nil, ErrInvalidCoverImport
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	connection, err := r.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	// Read all eligible columns before constructing the index: an incomplete
	// dictionary must not silently change correction or cross-library isolation.
	rows, err := connection.QueryContext(ctx, `SELECT id,COALESCE(title,''),COALESCE(editeur,''),COALESCE(auteur,''),COALESCE(tags,''),COALESCE(categorie,'') FROM defta WHERE library_id=? AND deleted_at IS NULL ORDER BY id LIMIT ?`, libraryID, MaxMatchingCatalogueBooks+1)
	if err != nil {
		return nil, fmt.Errorf("read scoped matching catalogue: %w", err)
	}
	catalogueBytes := 0
	books := make([]matchingCatalogueBook, 0)
	dictionary := map[string]int{}
	for rows.Next() {
		var book matchingCatalogueBook
		if err = rows.Scan(&book.id, &book.fields[0], &book.fields[1], &book.fields[2], &book.fields[3], &book.fields[4]); err != nil {
			rows.Close()
			return nil, err
		}
		for _, field := range book.fields {
			catalogueBytes += len(field)
			if len(field) > 65536 || catalogueBytes > 8*1024*1024 {
				rows.Close()
				return nil, ErrMatchingCatalogueLimit
			}
		}
		for i, field := range book.fields {
			book.fields[i] = strings.ToLower(ocr.NormalizeForMatching(field))
		}
		unique := map[string]bool{}
		for _, field := range book.fields {
			for _, word := range strings.Fields(field) {
				unique[word] = true
			}
		}
		for word := range unique {
			dictionary[word]++
		}
		books = append(books, book)
		if len(books) > MaxMatchingCatalogueBooks || len(dictionary) > MaxMatchingDictionaryWords {
			rows.Close()
			return nil, ErrMatchingCatalogueLimit
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	terms := ocr.MatchingTerms(text, dictionary)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(terms) == 0 {
		return []CoverImportCandidate{}, nil
	}
	var previousTempStore int
	if err = connection.QueryRowContext(ctx, "PRAGMA temp_store").Scan(&previousTempStore); err != nil {
		return nil, err
	}
	if _, err = connection.ExecContext(ctx, "PRAGMA temp_store=MEMORY"); err != nil {
		return nil, err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
		defer stop()
		_, _ = connection.ExecContext(cleanup, fmt.Sprintf("PRAGMA temp_store=%d", previousTempStore))
	}()
	table := fmt.Sprintf("cover_matching_%d", matchingTableID.Add(1)) // Generated identifier, never user text.
	if _, err = connection.ExecContext(ctx, `CREATE VIRTUAL TABLE temp.`+table+` USING fts5(book_id UNINDEXED,title,editeur,auteur,tags,categorie)`); err != nil {
		return nil, err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
		defer stop()
		_, _ = connection.ExecContext(cleanup, `DROP TABLE IF EXISTS temp.`+table)
	}()
	statement, err := connection.PrepareContext(ctx, `INSERT INTO temp.`+table+`(book_id,title,editeur,auteur,tags,categorie) VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return nil, err
	}
	for _, book := range books {
		if _, err = statement.ExecContext(ctx, book.id, book.fields[0], book.fields[1], book.fields[2], book.fields[3], book.fields[4]); err != nil {
			statement.Close()
			return nil, err
		}
	}
	statement.Close()
	query := strings.ReplaceAll(ocr.FTSQuery(strings.Join(terms, " ")), " AND ", " OR ")
	result, err := connection.QueryContext(ctx, `SELECT book_id,bm25(`+table+`,0.0,5.0,1.0,2.0,0.5,0.5) FROM temp.`+table+` WHERE `+table+` MATCH ? ORDER BY 2 ASC,book_id ASC LIMIT 5`, query)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	candidates := make([]CoverImportCandidate, 0, 5)
	for result.Next() {
		candidate := CoverImportCandidate{}
		if err = result.Scan(&candidate.BookID, &candidate.Score); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, result.Err()
}
