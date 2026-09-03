package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type OwnerHandler struct{ service *services.OwnerService }

func NewOwnerHandler(service *services.OwnerService) *OwnerHandler {
	return &OwnerHandler{service: service}
}

type createOwnerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Library  struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"library"`
}

type updateOwnerRequest struct {
	Username *string            `json:"username"`
	Email    *string            `json:"email"`
	Password *string            `json:"password"`
	Status   *models.UserStatus `json:"status"`
	Library  *struct {
		Name        *string               `json:"name"`
		Description *string               `json:"description"`
		Status      *models.LibraryStatus `json:"status"`
	} `json:"library"`
}

type resetOwnerPasswordRequest struct {
	Password string `json:"password"`
}

func (h *OwnerHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	owners, total, err := h.service.Search(r.Context(), r.URL.Query().Get("q"),
		strings.TrimSpace(r.URL.Query().Get("status")), strings.TrimSpace(r.URL.Query().Get("libraryStatus")), offset, limit)
	if err != nil {
		writeOwnerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": owners, "total": total, "offset": offset, "limit": limit,
	})
}

func (h *OwnerHandler) Get(w http.ResponseWriter, r *http.Request) {
	owner, err := h.service.Find(r.Context(), r.PathValue("id"))
	if err != nil {
		writeOwnerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, owner)
}

func (h *OwnerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request createOwnerRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeOwnerError(w, services.ErrInvalidOwner)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	owner, err := h.service.Create(r.Context(), models.OwnerCreateInput{
		Username: request.Username, Email: request.Email, Password: request.Password,
		LibraryName: request.Library.Name, LibraryDescription: request.Library.Description,
	}, claims.Subject)
	if err != nil {
		writeOwnerError(w, err)
		return
	}
	w.Header().Set("Location", "/api/admin/owners/"+owner.ID)
	writeAuthJSON(w, http.StatusCreated, owner)
}

func (h *OwnerHandler) Update(w http.ResponseWriter, r *http.Request) {
	var request updateOwnerRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeOwnerError(w, services.ErrInvalidOwner)
		return
	}
	if request.Username == nil && request.Email == nil && request.Password == nil &&
		request.Status == nil && request.Library == nil {
		writeOwnerError(w, services.ErrInvalidOwner)
		return
	}
	input := models.OwnerUpdateInput{
		Username: request.Username, Email: request.Email, Password: request.Password, Status: request.Status,
	}
	if request.Library != nil {
		input.LibraryName = request.Library.Name
		input.LibraryDescription = request.Library.Description
		input.LibraryStatus = request.Library.Status
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	owner, err := h.service.Update(r.Context(), r.PathValue("id"), input, claims.Subject)
	if err != nil {
		writeOwnerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, owner)
}

func (h *OwnerHandler) Disable(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Disable(r.Context(), r.PathValue("id"), claims.Subject); err != nil {
		writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OwnerHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Unlock(r.Context(), r.PathValue("id"), claims.Subject); err != nil {
		writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OwnerHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Reactivate(r.Context(), r.PathValue("id"), claims.Subject); err != nil {
		writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OwnerHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var request resetOwnerPasswordRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil || strings.TrimSpace(request.Password) == "" {
		writeOwnerError(w, services.ErrInvalidOwner)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.ResetPassword(r.Context(), r.PathValue("id"), request.Password, claims.Subject); err != nil {
		writeOwnerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeOwnerJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return services.ErrInvalidOwner
	}
	return nil
}

func writeOwnerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrOwnerNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "owner_not_found", "message": "Library owner not found"})
	case errors.Is(err, repositories.ErrOwnerConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "owner_conflict", "message": "Username or email already exists"})
	case errors.Is(err, repositories.ErrOwnerNotLocked):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "owner_not_locked", "message": "Library owner is not locked"})
	case errors.Is(err, repositories.ErrOwnerNotDisabled):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "owner_not_disabled", "message": "Library owner is not disabled"})
	case errors.Is(err, services.ErrPasswordReused):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_new_password", "message": err.Error()})
	case errors.Is(err, services.ErrInvalidOwner), errors.Is(err, auth.ErrPasswordTooShort):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Owner operation failed"})
	}
}
