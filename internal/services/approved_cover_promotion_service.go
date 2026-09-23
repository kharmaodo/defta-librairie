package services

import (
	"context"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type approvedSourcePromoter interface {
	CopySource(ctx context.Context, fromKey, toKey string, size int64, contentType string) error
	DeleteObject(ctx context.Context, key string) error
}

type ApprovedCoverPromotionService struct {
	submissions *repositories.BookSubmissionRepository
	covers      *repositories.CoverRepository
	store       approvedSourcePromoter
	newID       func() (string, error)
	now         func() time.Time
}

func NewApprovedCoverPromotionService(
	submissions *repositories.BookSubmissionRepository,
	coverRepository *repositories.CoverRepository,
	store approvedSourcePromoter,
) *ApprovedCoverPromotionService {
	return &ApprovedCoverPromotionService{
		submissions: submissions, covers: coverRepository, store: store,
		newID: identity.NewID, now: time.Now,
	}
}

func (s *ApprovedCoverPromotionService) RunOnce(ctx context.Context, limit int) error {
	if s == nil || s.submissions == nil || s.covers == nil || s.store == nil || s.newID == nil {
		return ErrInvalidModerationWorker
	}
	items, err := s.submissions.ApprovedWithoutCover(ctx, limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = s.promote(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *ApprovedCoverPromotionService) promote(ctx context.Context, item repositories.ApprovedSubmissionForPromotion) error {
	coverID, err := s.newID()
	if err != nil {
		return err
	}
	eventID, err := s.newID()
	if err != nil {
		return err
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	extension := "jpg"
	if item.SourceFormat == "png" {
		extension = "png"
	}
	target := "sources/" + item.LibraryID + "/" + strconv.Itoa(item.BookID) + "/" + coverID + "." + extension
	if err = s.store.CopySource(ctx, item.SourceObjectKey, target, item.SourceSize, item.SourceContentType); err != nil {
		return err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(map[string]interface{}{
		"schemaVersion": 1, "eventId": eventID, "coverId": coverID,
		"bookId": item.BookID, "libraryId": item.LibraryID,
		"sourceObjectKey": target, "attempt": 1,
	})
	if err != nil {
		_ = s.store.DeleteObject(ctx, target)
		return err
	}
	err = s.covers.CreatePending(ctx, repositories.PendingCover{
		ID: coverID, BookID: item.BookID, LibraryID: item.LibraryID,
		SourceObjectKey: target, SourceContentType: item.SourceContentType,
		SourceFormat: item.SourceFormat, SourceWidth: item.SourceWidth,
		SourceHeight: item.SourceHeight, SourceSize: item.SourceSize,
	}, eventID, string(payload), item.ActorUserID, auditID, now)
	if err != nil {
		_ = s.store.DeleteObject(ctx, target)
		return fmt.Errorf("create promoted cover: %w", err)
	}
	_ = s.store.DeleteObject(ctx, item.SourceObjectKey)
	return nil
}

var _ approvedSourcePromoter = (*covers.MinIOProcessingStore)(nil)
