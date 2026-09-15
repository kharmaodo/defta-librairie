package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"defta-librairie/internal/repositories"
)

var ErrCoverEventPublish = errors.New("cover event publication failed")

type CoverMessagePublisher interface {
	Publish(ctx context.Context, subject, eventID string, payload []byte) error
}

type CoverOutboxStore interface {
	ClaimNext(ctx context.Context, workerID string, now, leaseUntil time.Time) (repositories.CoverOutboxEvent, error)
	MarkPublished(ctx context.Context, eventID, workerID string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, eventID, workerID string, availableAt time.Time, lastError string) error
}

type CoverOutboxPublisher struct {
	store         CoverOutboxStore
	messages      CoverMessagePublisher
	workerID      string
	subject       string
	leaseDuration time.Duration
	baseRetry     time.Duration
	maxRetry      time.Duration
	now           func() time.Time
}

func NewCoverOutboxPublisher(
	store CoverOutboxStore,
	messages CoverMessagePublisher,
	workerID string,
	subject string,
) (*CoverOutboxPublisher, error) {
	if store == nil || messages == nil || workerID == "" || subject == "" {
		return nil, fmt.Errorf("invalid cover outbox publisher configuration")
	}
	return &CoverOutboxPublisher{
		store: store, messages: messages, workerID: workerID, subject: subject,
		leaseDuration: time.Minute,
		baseRetry:     5 * time.Second,
		maxRetry:      5 * time.Minute,
		now:           time.Now,
	}, nil
}

// PublishAvailable publie au plus limit événements. Un message est marqué
// publié uniquement après l'acquittement du transport.
func (p *CoverOutboxPublisher) PublishAvailable(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		return 0, nil
	}
	published := 0
	for published < limit {
		if err := ctx.Err(); err != nil {
			return published, err
		}
		now := p.now().UTC()
		event, err := p.store.ClaimNext(ctx, p.workerID, now, now.Add(p.leaseDuration))
		if errors.Is(err, repositories.ErrCoverOutboxEmpty) {
			return published, nil
		}
		if err != nil {
			return published, fmt.Errorf("claim cover event: %w", err)
		}

		err = p.messages.Publish(ctx, p.subject, event.EventID, []byte(event.Payload))
		if err != nil {
			nextAttempt := now.Add(p.retryDelay(event.Attempts))
			recordErr := p.store.MarkFailed(
				ctx, event.EventID, p.workerID, nextAttempt, coverPublishError(err),
			)
			if recordErr != nil {
				return published, errors.Join(
					fmt.Errorf("%w: %v", ErrCoverEventPublish, err),
					fmt.Errorf("record cover publication failure: %w", recordErr),
				)
			}
			return published, fmt.Errorf("%w: %v", ErrCoverEventPublish, err)
		}
		if err = p.store.MarkPublished(ctx, event.EventID, p.workerID, now); err != nil {
			return published, fmt.Errorf("acknowledge cover event: %w", err)
		}
		published++
	}
	return published, nil
}

func (p *CoverOutboxPublisher) retryDelay(previousAttempts int) time.Duration {
	if previousAttempts < 0 {
		previousAttempts = 0
	}
	delay := p.baseRetry
	for i := 0; i < previousAttempts && delay < p.maxRetry; i++ {
		if delay > p.maxRetry/2 {
			return p.maxRetry
		}
		delay *= 2
	}
	if delay > p.maxRetry {
		return p.maxRetry
	}
	return delay
}

func coverPublishError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.NewReplacer(`\n`, " ", `\r`, " ", `\t`, " ").Replace(err.Error())
	message = strings.Join(strings.Fields(message), " ")
	const maxRunes = 512
	runes := []rune(message)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return message
}
