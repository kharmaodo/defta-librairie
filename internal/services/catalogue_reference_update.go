package services

import (
 "context"
 "defta-librairie/internal/auth"
 "defta-librairie/internal/identity"
 "defta-librairie/internal/models"
 "strconv"
 "strings"
 "time"
)

func (s *CatalogueReferenceService) Update(ctx context.Context, claims *auth.Claims, kind, rawID string, value models.CatalogueReference) error {
 if claims==nil || claims.Role!=models.RoleSuperAdminRoot { return ErrBookForbidden }
 id,err:=strconv.ParseInt(rawID,10,64);if err!=nil||id<1{return ErrInvalidCatalogueReference}
 value.Code=strings.TrimSpace(strings.ToLower(value.Code));value.Name=strings.TrimSpace(value.Name);value.Arabic=strings.TrimSpace(value.Arabic);value.French=strings.TrimSpace(value.French);value.English=strings.TrimSpace(value.English)
 if (kind!="category"&&kind!="publisher")||value.Code==""||value.Name==""||value.Arabic==""||value.French==""||value.English=="" {return ErrInvalidCatalogueReference}
 auditID,err:=identity.NewID();if err!=nil{return err};value.UpdatedAt=time.Now().UTC().Format(time.RFC3339Nano)
 return s.repository.Update(ctx,kind,id,value,claims.Subject,auditID)
}
