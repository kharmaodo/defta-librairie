package services

import (
 "context"
 "defta-librairie/internal/auth"
 "defta-librairie/internal/models"
 "errors"
 "testing"
)

func TestCatalogueReferenceMutationsRequireRoot(t *testing.T) {
 service:=NewCatalogueReferenceService(nil)
 owner:=&auth.Claims{Role:models.RoleOwnerLibrary}
 value:=models.CatalogueReference{Code:"fiqh",Name:"fqh",Arabic:"الفقه",French:"Fiqh",English:"Fiqh"}
 if err:=service.Create(context.Background(),owner,"category",value);!errors.Is(err,ErrBookForbidden){t.Fatalf("create err=%v",err)}
 if err:=service.Update(context.Background(),owner,"category","1",value);!errors.Is(err,ErrBookForbidden){t.Fatalf("update err=%v",err)}
 if err:=service.SetActive(context.Background(),owner,"category","1",false);!errors.Is(err,ErrBookForbidden){t.Fatalf("disable err=%v",err)}
}
