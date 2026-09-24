package services

import (
 "context"
 "defta-librairie/internal/auth"
 "defta-librairie/internal/identity"
 "strconv"
 "time"
)

func (s *CatalogueReferenceService) SetActive(ctx context.Context, claims *auth.Claims, kind, rawID string, active bool) error {
 if claims==nil || claims.Role!=auth.RoleSuperAdminRoot { return ErrBookForbidden }
 if kind!="category" && kind!="publisher" { return ErrInvalidCatalogueReference }
 id,err:=strconv.ParseInt(rawID,10,64);if err!=nil||id<1{return ErrInvalidCatalogueReference}
 auditID,err:=identity.NewID();if err!=nil{return err}
 return s.repository.SetActive(ctx,kind,id,active,claims.Subject,auditID,time.Now().UTC().Format(time.RFC3339Nano))
}
