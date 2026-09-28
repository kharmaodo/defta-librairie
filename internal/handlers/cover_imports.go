package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
)

type coverImportCreator interface { Create(context.Context,*auth.Claims,string,string,[]services.CoverImportFile)(models.CoverImport,error); List(context.Context,*auth.Claims,string,int)([]models.CoverImport,error) }
type CoverImportHandler struct { service coverImportCreator; enabled bool; maxFileBytes int64 }
func NewCoverImportHandler(service coverImportCreator,enabled bool,maxFileBytes int64)*CoverImportHandler{if maxFileBytes<1{maxFileBytes=10*1024*1024};return &CoverImportHandler{service:service,enabled:enabled,maxFileBytes:maxFileBytes}}

func (h *CoverImportHandler) Create(w http.ResponseWriter,r *http.Request){
	if !h.enabled||h.service==nil{writeAuthJSON(w,http.StatusServiceUnavailable,map[string]string{"error":"covers_disabled"});return}
	r.Body=http.MaxBytesReader(w,r.Body,services.CoverImportMaxBytes+multipartEnvelopeAllowance)
	if err:=r.ParseMultipartForm(services.CoverImportMaxBytes+multipartEnvelopeAllowance);err!=nil{writeCoverImportError(w,covers.ErrTooLarge);return};if r.MultipartForm!=nil{defer r.MultipartForm.RemoveAll()}
	files:=r.MultipartForm.File["covers"];if len(files)<1||len(files)>services.CoverImportMaxFiles{writeCoverImportError(w,services.ErrInvalidBook);return}
	items:=make([]services.CoverImportFile,0,len(files));for _,header:=range files {if header.Size<1||header.Size>h.maxFileBytes{writeCoverImportError(w,covers.ErrTooLarge);return};file,err:=header.Open();if err!=nil{writeCoverImportError(w,services.ErrInvalidBook);return};defer file.Close();items=append(items,services.CoverImportFile{DeclaredContentType:header.Header.Get("Content-Type"),Body:file})}
	claims,_:=auth.ClaimsFromContext(r.Context());result,err:=h.service.Create(r.Context(),claims,r.FormValue("libraryId"),r.Header.Get("Idempotency-Key"),items);if err!=nil{writeCoverImportError(w,err);return};w.Header().Set("Location","/api/manage/cover-imports/"+result.ID);writeAuthJSON(w,http.StatusAccepted,result)
}
func(h *CoverImportHandler)List(w http.ResponseWriter,r *http.Request){if !h.enabled||h.service==nil{writeAuthJSON(w,http.StatusServiceUnavailable,map[string]string{"error":"covers_disabled"});return};limit:=30;if value:=r.URL.Query().Get("limit");value!=""{if parsed,err:=strconv.Atoi(value);err==nil{limit=parsed}};claims,_:=auth.ClaimsFromContext(r.Context());items,err:=h.service.List(r.Context(),claims,r.URL.Query().Get("libraryId"),limit);if err!=nil{writeCoverImportError(w,err);return};writeAuthJSON(w,http.StatusOK,map[string]any{"results":items,"total":len(items)})}
func writeCoverImportError(w http.ResponseWriter,err error){switch{case errors.Is(err,repositories.ErrCoverImportQuota):writeAuthJSON(w,http.StatusTooManyRequests,map[string]string{"error":"cover_import_quota_exceeded","message":"Cover import quota exceeded"});case errors.Is(err,services.ErrBookForbidden):writeAuthJSON(w,http.StatusForbidden,map[string]string{"error":"forbidden","message":"Insufficient permissions"});case errors.Is(err,covers.ErrTooLarge):writeAuthJSON(w,http.StatusRequestEntityTooLarge,map[string]string{"error":"cover_import_too_large","message":err.Error()});case errors.Is(err,covers.ErrEmpty),errors.Is(err,covers.ErrUnsupportedFormat),errors.Is(err,covers.ErrContentTypeMismatch),errors.Is(err,covers.ErrCorruptImage),errors.Is(err,covers.ErrTooManyPixels),errors.Is(err,services.ErrInvalidBook),errors.Is(err,repositories.ErrInvalidCoverImport):writeAuthJSON(w,http.StatusUnprocessableEntity,map[string]string{"error":"invalid_cover_import","message":"Invalid cover import"});case errors.Is(err,services.ErrCoversDisabled),errors.Is(err,covers.ErrStoreUnavailable),errors.Is(err,services.ErrCoverPersistence):writeAuthJSON(w,http.StatusServiceUnavailable,map[string]string{"error":"cover_import_unavailable","message":"Cover import unavailable"});default:writeAuthJSON(w,http.StatusInternalServerError,map[string]string{"error":"internal_error","message":"Cover import failed"})}}

var _ = multipart.FileHeader{}
