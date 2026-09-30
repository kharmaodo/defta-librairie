package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"io"
	"net/http"
)

type coverImportReviewer interface {
	Job(context.Context, *auth.Claims, string, string) (repositories.CoverImportReviewJob, error)
	Decide(context.Context, *auth.Claims, string, string, string, string, int) (repositories.CoverImportReviewJob, error)
	DecideQuarantine(context.Context, *auth.Claims, string, string, string) (repositories.CoverImportReviewJob, error)
	OpenSource(context.Context, *auth.Claims, string, string) (io.ReadCloser, string, error)
}

func (h *CoverImportReviewHandler) Source(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	claims, _ := auth.ClaimsFromContext(r.Context())
	reader, contentType, err := h.service.OpenSource(r.Context(), claims, r.PathValue("id"), r.URL.Query().Get("libraryId"))
	if err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", contentType)
	_, _ = io.Copy(w, io.LimitReader(reader, 10485761))
}

type CoverImportReviewHandler struct {
	service coverImportReviewer
	enabled bool
}

func NewCoverImportReviewHandler(service coverImportReviewer, enabled bool) *CoverImportReviewHandler {
	return &CoverImportReviewHandler{service: service, enabled: enabled}
}

type coverImportReviewResponse struct {
	ID           string                                    `json:"id"`
	LibraryID    string                                    `json:"libraryId"`
	Status       string                                    `json:"status"`
	NSFWDecision string                                    `json:"nsfwDecision"`
	Candidates   []repositories.CoverImportReviewCandidate `json:"candidates"`
}

func safeCoverImportReview(job repositories.CoverImportReviewJob) coverImportReviewResponse {
	return coverImportReviewResponse{ID: job.ID, LibraryID: job.LibraryID, Status: job.Status, NSFWDecision: job.NSFWDecision, Candidates: job.Candidates}
}

func (h *CoverImportReviewHandler) Job(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	job, err := h.service.Job(r.Context(), claims, r.PathValue("id"), r.URL.Query().Get("libraryId"))
	if err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, safeCoverImportReview(job))
}

type coverImportDecisionRequest struct {
	LibraryID string `json:"libraryId"`
	Action    string `json:"action"`
	BookID    int    `json:"bookId"`
	Reason    string `json:"reason"`
}

func (h *CoverImportReviewHandler) Decide(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
		return
	}
	var request coverImportDecisionRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeCoverImportReviewError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	job, err := h.service.Decide(r.Context(), claims, r.PathValue("id"), request.LibraryID, request.Action, request.Reason, request.BookID)
	if err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusAccepted, safeCoverImportReview(job))
}

type coverImportQuarantineRequest struct {
	LibraryID string `json:"libraryId"`
	Decision  string `json:"decision"`
}

func (h *CoverImportReviewHandler) DecideQuarantine(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
		return
	}
	var request coverImportQuarantineRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeCoverImportReviewError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	job, err := h.service.DecideQuarantine(r.Context(), claims, r.PathValue("id"), request.LibraryID, request.Decision)
	if err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusAccepted, safeCoverImportReview(job))
}

func writeCoverImportReviewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, repositories.ErrCoverImportJobNotFound), errors.Is(err, repositories.ErrCoverImportCandidateNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "cover_import_not_found"})
	case errors.Is(err, repositories.ErrCoverImportReviewState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "cover_import_not_actionable"})
	case errors.Is(err, services.ErrInvalidBook), errors.Is(err, repositories.ErrInvalidCoverImport):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_cover_import_decision"})
	case errors.Is(err, services.ErrCoversDisabled):
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
	default:
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cover_import_review_unavailable"})
	}
}

type manualCoverReviewer interface {
	SearchCandidates(context.Context, *auth.Claims, string, string, string) ([]repositories.CoverImportReviewCandidate, error)
	DismissCandidate(context.Context, *auth.Claims, string, string, int) error
}

func (h *CoverImportReviewHandler) SearchCandidates(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.service.(manualCoverReviewer)
	if !h.enabled || !ok {
		writeCoverImportReviewError(w, services.ErrCoversDisabled)
		return
	}
	var request struct {
		LibraryID string `json:"libraryId"`
		Query     string `json:"query"`
	}
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeCoverImportReviewError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	result, err := svc.SearchCandidates(r.Context(), claims, r.PathValue("id"), request.LibraryID, request.Query)
	if err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeAuthJSON(w, http.StatusOK, map[string]any{"results": result})
}
func (h *CoverImportReviewHandler) DismissCandidate(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.service.(manualCoverReviewer)
	if !h.enabled || !ok {
		writeCoverImportReviewError(w, services.ErrCoversDisabled)
		return
	}
	var request struct {
		LibraryID string `json:"libraryId"`
		BookID    int    `json:"bookId"`
	}
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeCoverImportReviewError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := svc.DismissCandidate(r.Context(), claims, r.PathValue("id"), request.LibraryID, request.BookID); err != nil {
		writeCoverImportReviewError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "DISMISSED"})
}
