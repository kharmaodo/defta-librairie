package services

import (
 "context"
 "defta-librairie/internal/auth"
 "defta-librairie/internal/identity"
 "defta-librairie/internal/models"
 "strings"
 "time"
)

func (s *CatalogueReferenceService) Create(ctx context.Context, claims *auth.Claims, kind string, value models.CatalogueReference) error {
 if claims == nil || claims.Role != auth.RoleSuperAdminRoot { return ErrBookForbidden }
 value.Code=strings.TrimSpace(strings.ToLower(value.Code)); value.Name=strings.TrimSpace(value.Name)
 value.Arabic=strings.TrimSpace(value.Arabic); value.French=strings.TrimSpace(value.French); value.English=strings.TrimSpace(value.English)
 if kind!="category"&&kind!="publisher" || value.Code=="" || value.Name=="" || value.Arabic=="" || value.French=="" || value.English=="" { return ErrInvalidCatalogueReference }
 now:=time.Now().UTC().Format(time.RFC3339Nano); value.CreatedAt=now; value.UpdatedAt=now
 auditID,err:=identity.NewID(); if err!=nil{return err}
 return s.repository.Create(ctx,kind,value,claims.Subject,auditID)
}
