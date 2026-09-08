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

var ErrInvalidSupplierReturn = errors.New("invalid supplier return data")

type SupplierReturnService struct { repository *repositories.SupplierReturnRepository; now func() time.Time }

func NewSupplierReturnService(repository *repositories.SupplierReturnRepository) *SupplierReturnService { return &SupplierReturnService{repository:repository,now:time.Now} }

func(s *SupplierReturnService)List(ctx context.Context,claims *auth.Claims,requestedLibrary string,filter models.SupplierReturnFilter,offset,limit int)([]models.SupplierReturn,int,error){
	filter.Status=models.SupplierReturnStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))));filter.PurchaseID=strings.TrimSpace(filter.PurchaseID);filter.SupplierID=strings.TrimSpace(filter.SupplierID)
	if filter.Status!=""&&filter.Status!=models.SupplierReturnStatusDraft&&filter.Status!=models.SupplierReturnStatusShipped&&filter.Status!=models.SupplierReturnStatusCancelled{return nil,0,ErrInvalidSupplierReturn}
	if !validSaleDate(filter.From)||!validSaleDate(filter.To)||(filter.From!=""&&filter.To!=""&&filter.From>filter.To){return nil,0,ErrInvalidSupplierReturn}
	libraryID,err:=resolveBookScope(claims,strings.TrimSpace(requestedLibrary),false);if err!=nil{return nil,0,err};if offset<0{offset=0};if limit<1||limit>100{limit=30};return s.repository.List(ctx,libraryID,filter,offset,limit)
}
func(s *SupplierReturnService)Find(ctx context.Context,claims *auth.Claims,id string)(models.SupplierReturn,error){if strings.TrimSpace(id)==""{return models.SupplierReturn{},ErrInvalidSupplierReturn};libraryID,err:=resolveBookScope(claims,"",false);if err!=nil{return models.SupplierReturn{},err};return s.repository.Find(ctx,id,libraryID)}
func(s *SupplierReturnService)Create(ctx context.Context,claims *auth.Claims,input models.SupplierReturnInput)(models.SupplierReturn,error){libraryID,err:=resolveBookScope(claims,strings.TrimSpace(input.LibraryID),true);if err!=nil{return models.SupplierReturn{},err};if validateSupplierReturn(input,false)!=nil{return models.SupplierReturn{},ErrInvalidSupplierReturn};id,lineIDs,auditID,err:=supplierReturnIDs(len(input.Lines));if err!=nil{return models.SupplierReturn{},err};now:=s.now().UTC();compact:=strings.ToUpper(strings.ReplaceAll(id,"-",""));value:=models.SupplierReturn{ID:id,LibraryID:libraryID,PurchaseID:strings.TrimSpace(input.PurchaseID),Reference:fmt.Sprintf("RF-%s-%s",now.Format("20060102"),compact[:8]),SupplierReference:strings.TrimSpace(input.SupplierReference),Reason:strings.TrimSpace(input.Reason),CreatedBy:claims.Subject};return s.repository.Create(ctx,value,input.Lines,lineIDs,auditID,now.Format(time.RFC3339Nano))}
func(s *SupplierReturnService)Update(ctx context.Context,claims *auth.Claims,id string,input models.SupplierReturnInput)(models.SupplierReturn,error){if strings.TrimSpace(id)==""||validateSupplierReturn(input,true)!=nil{return models.SupplierReturn{},ErrInvalidSupplierReturn};libraryID,err:=resolveBookScope(claims,"",false);if err!=nil{return models.SupplierReturn{},err};_,lineIDs,auditID,err:=supplierReturnIDs(len(input.Lines));if err!=nil{return models.SupplierReturn{},err};return s.repository.Update(ctx,id,libraryID,strings.TrimSpace(input.SupplierReference),strings.TrimSpace(input.Reason),input.Lines,lineIDs,input.Version,claims.Subject,auditID,s.now().UTC().Format(time.RFC3339Nano))}
func(s *SupplierReturnService)Cancel(ctx context.Context,claims *auth.Claims,id string,version int)(models.SupplierReturn,error){if strings.TrimSpace(id)==""||version<1{return models.SupplierReturn{},ErrInvalidSupplierReturn};libraryID,err:=resolveBookScope(claims,"",false);if err!=nil{return models.SupplierReturn{},err};auditID,err:=identity.NewID();if err!=nil{return models.SupplierReturn{},err};return s.repository.Cancel(ctx,id,libraryID,version,claims.Subject,auditID,s.now().UTC().Format(time.RFC3339Nano))}
func validateSupplierReturn(input models.SupplierReturnInput,update bool)error{reason:=strings.TrimSpace(input.Reason);reference:=strings.TrimSpace(input.SupplierReference);if (!update&&strings.TrimSpace(input.PurchaseID)=="")||len([]rune(reason))<3||len([]rune(reason))>1000||len([]rune(reference))>160||len(input.Lines)<1||len(input.Lines)>100||(update&&input.Version<1){return ErrInvalidSupplierReturn};seen:=map[string]bool{};for _,line:=range input.Lines{id:=strings.TrimSpace(line.PurchaseLineID);if id==""||line.Quantity<1||line.Quantity>100000||seen[id]{return ErrInvalidSupplierReturn};seen[id]=true};return nil}
func supplierReturnIDs(count int)(string,[]string,string,error){id,err:=identity.NewID();if err!=nil{return "",nil,"",err};lines:=make([]string,count);for i:=range lines{if lines[i],err=identity.NewID();err!=nil{return "",nil,"",err}};audit,err:=identity.NewID();return id,lines,audit,err}

func(s *SupplierReturnService)Ship(ctx context.Context,claims *auth.Claims,id string,version int)(models.SupplierReturn,error){
 if strings.TrimSpace(id)==""||version<1{return models.SupplierReturn{},ErrInvalidSupplierReturn}
 libraryID,err:=resolveBookScope(claims,"",false);if err!=nil{return models.SupplierReturn{},err}
 return s.repository.Ship(ctx,strings.TrimSpace(id),libraryID,version,claims.Subject,s.now().UTC().Format(time.RFC3339Nano))
}
