package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidCustomerReturn = errors.New("invalid customer return data")

type CustomerReturnService struct {
	repository *repositories.CustomerReturnRepository
	now func() time.Time
}

func NewCustomerReturnService(repository *repositories.CustomerReturnRepository) *CustomerReturnService {
	return &CustomerReturnService{repository: repository, now: time.Now}
}

func (s *CustomerReturnService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string,
	filter models.CustomerReturnFilter, offset, limit int) ([]models.CustomerReturn, int, error) {
	filter.Status = models.CustomerReturnStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	filter.SaleID, filter.CustomerID = strings.TrimSpace(filter.SaleID), strings.TrimSpace(filter.CustomerID)
	if filter.Status != "" && filter.Status != models.CustomerReturnStatusDraft &&
		filter.Status != models.CustomerReturnStatusCompleted && filter.Status != models.CustomerReturnStatusCancelled {
		return nil, 0, ErrInvalidCustomerReturn
	}
	if !validSaleDate(filter.From) || !validSaleDate(filter.To) ||
		(filter.From != "" && filter.To != "" && filter.From > filter.To) { return nil, 0, ErrInvalidCustomerReturn }
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil { return nil, 0, err }
	if offset < 0 { offset = 0 }; if limit < 1 || limit > 100 { limit = 30 }
	return s.repository.List(ctx, libraryID, filter, offset, limit)
}

func (s *CustomerReturnService) Find(ctx context.Context, claims *auth.Claims, id string) (models.CustomerReturn, error) {
	if strings.TrimSpace(id) == "" { return models.CustomerReturn{}, ErrInvalidCustomerReturn }
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.CustomerReturn{}, err }
	return s.repository.Find(ctx, id, libraryID)
}

func (s *CustomerReturnService) Create(ctx context.Context, claims *auth.Claims, input models.CustomerReturnInput) (models.CustomerReturn, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil { return models.CustomerReturn{}, err }
	if err = validateCustomerReturn(input, false); err != nil { return models.CustomerReturn{}, err }
	returnID, lineIDs, auditID, err := customerReturnIDs(len(input.Lines))
	if err != nil { return models.CustomerReturn{}, err }
	now := s.now().UTC(); compact := strings.ToUpper(strings.ReplaceAll(returnID, "-", ""))
	value := models.CustomerReturn{ID:returnID, LibraryID:libraryID, SaleID:strings.TrimSpace(input.SaleID),
		Reference:fmt.Sprintf("R-%s-%s", now.Format("20060102"), compact[:8]), Reason:strings.TrimSpace(input.Reason),
		Resolution:input.Resolution, CreatedBy:claims.Subject}
	return s.repository.Create(ctx, value, input.Lines, lineIDs, auditID, now.Format(time.RFC3339Nano))
}

func (s *CustomerReturnService) Update(ctx context.Context, claims *auth.Claims, id string, input models.CustomerReturnInput) (models.CustomerReturn, error) {
	if strings.TrimSpace(id) == "" || validateCustomerReturn(input, true) != nil { return models.CustomerReturn{}, ErrInvalidCustomerReturn }
	libraryID, err := resolveBookScope(claims, "", false); if err != nil { return models.CustomerReturn{}, err }
	_, lineIDs, auditID, err := customerReturnIDs(len(input.Lines)); if err != nil { return models.CustomerReturn{}, err }
	return s.repository.Update(ctx, id, libraryID, strings.TrimSpace(input.Reason), input.Resolution, input.Lines,
		lineIDs, input.Version, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *CustomerReturnService) Complete(ctx context.Context, claims *auth.Claims, id string, version int) (models.CustomerReturn, error) {
	return s.transition(ctx, claims, id, version, models.CustomerReturnStatusCompleted)
}
func (s *CustomerReturnService) Cancel(ctx context.Context, claims *auth.Claims, id string, version int) (models.CustomerReturn, error) {
	return s.transition(ctx, claims, id, version, models.CustomerReturnStatusCancelled)
}
func (s *CustomerReturnService) transition(ctx context.Context, claims *auth.Claims, id string, version int, target models.CustomerReturnStatus) (models.CustomerReturn, error) {
	if strings.TrimSpace(id) == "" || version < 1 { return models.CustomerReturn{}, ErrInvalidCustomerReturn }
	libraryID, err := resolveBookScope(claims, "", false); if err != nil { return models.CustomerReturn{}, err }
	value, err := s.repository.Find(ctx, id, libraryID); if err != nil { return models.CustomerReturn{}, err }
	movements, inventoryAudits := []string{}, []string{}
	if target == models.CustomerReturnStatusCompleted {
		movements, inventoryAudits = make([]string,len(value.Lines)), make([]string,len(value.Lines))
		for i := range value.Lines { if movements[i],err=identity.NewID();err!=nil{return models.CustomerReturn{},err}; if inventoryAudits[i],err=identity.NewID();err!=nil{return models.CustomerReturn{},err} }
	}
	auditID, err := identity.NewID(); if err != nil { return models.CustomerReturn{}, err }
	return s.repository.Transition(ctx,id,libraryID,claims.Subject,version,target,movements,inventoryAudits,auditID,s.now().UTC().Format(time.RFC3339Nano))
}

func validateCustomerReturn(input models.CustomerReturnInput, update bool) error {
	input.Reason = strings.TrimSpace(input.Reason)
	if (!update && strings.TrimSpace(input.SaleID)=="") || len(input.Reason)<3 || len(input.Reason)>1000 || len(input.Lines)<1 || len(input.Lines)>100 ||
		(input.Resolution!=models.CustomerReturnResolutionRefund && input.Resolution!=models.CustomerReturnResolutionCreditNote) || (update && input.Version<1) { return ErrInvalidCustomerReturn }
	seen:=map[string]bool{}; for _,line:=range input.Lines { id:=strings.TrimSpace(line.SaleLineID); if id=="" || line.Quantity<1 || line.Quantity>100000 || seen[id] { return ErrInvalidCustomerReturn }; seen[id]=true }
	return nil
}
func customerReturnIDs(count int) (string,[]string,string,error) {
	id,err:=identity.NewID(); if err!=nil{return "",nil,"",err}; lines:=make([]string,count)
	for i:=range lines { if lines[i],err=identity.NewID();err!=nil{return "",nil,"",err} }
	audit,err:=identity.NewID(); return id,lines,audit,err
}
