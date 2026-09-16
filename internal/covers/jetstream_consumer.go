package covers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	ErrInvalidCoverMessage      = errors.New("invalid cover processing message")
	ErrPermanentCoverProcessing = errors.New("permanent cover processing failure")
)

type ProcessingEvent struct {
	SchemaVersion   int    `json:"schemaVersion"`
	EventID         string `json:"eventId"`
	CoverID         string `json:"coverId"`
	BookID          int    `json:"bookId"`
	LibraryID       string `json:"libraryId"`
	SourceObjectKey string `json:"sourceObjectKey"`
	Attempt         int    `json:"attempt"`
}

type CoverEventHandler interface {
	Handle(ctx context.Context, event ProcessingEvent) error
}

type CoverEventFailureHandler interface {
	MarkFailed(ctx context.Context, event ProcessingEvent, cause error) error
}

type JetStreamConsumer struct {
	connection *nats.Conn
	jetStream  nats.JetStreamContext
	subscription *nats.Subscription
	retryDelay time.Duration
	maxDeliver int
}

func NewJetStreamConsumer(
	ctx context.Context,
	url string,
	user string,
	password string,
	streamName string,
	subject string,
	durable string,
	maxDeliver int,
) (*JetStreamConsumer, error) {
	if url == "" || streamName == "" || subject == "" || durable == "" || maxDeliver < 1 {
		return nil, fmt.Errorf("invalid JetStream consumer configuration")
	}
	if (user == "") != (password == "") {
		return nil, fmt.Errorf("NATS user and password must be configured together")
	}
	options := []nats.Option{
		nats.Name("defta-cover-worker"),
		nats.Timeout(5 * time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
	}
	if user != "" {
		options = append(options, nats.UserInfo(user, password))
	}
	connection, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("connect NATS consumer: %w", err)
	}
	jetStream, err := connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open consumer JetStream context: %w", err)
	}
	info, err := jetStream.StreamInfo(streamName, nats.Context(ctx))
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("inspect consumer stream: %w", err)
	}
	if err = validateCoverStream(info.Config, subject); err != nil {
		connection.Close()
		return nil, fmt.Errorf("incompatible consumer stream: %w", err)
	}
	subscription, err := jetStream.PullSubscribe(
		subject,
		durable,
		nats.BindStream(streamName),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(maxDeliver),
	)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("create durable cover consumer: %w", err)
	}
	return &JetStreamConsumer{
		connection: connection,
		jetStream: jetStream,
		subscription: subscription,
		retryDelay: 5 * time.Second,
		maxDeliver: maxDeliver,
	}, nil
}

// FetchAndHandle traite au plus batch messages. Un timeout sans message est un
// état normal. Un message métier invalide est terminé et ne sera pas rejoué.
func (c *JetStreamConsumer) FetchAndHandle(
	ctx context.Context,
	batch int,
	wait time.Duration,
	handler CoverEventHandler,
) (int, error) {
	if c == nil || c.subscription == nil || handler == nil || batch < 1 || wait <= 0 {
		return 0, fmt.Errorf("invalid cover consumer execution")
	}
	messages, err := c.subscription.Fetch(batch, nats.MaxWait(wait))
	if errors.Is(err, nats.ErrTimeout) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("fetch cover messages: %w", err)
	}
	handled := 0
	for _, message := range messages {
		if err = ctx.Err(); err != nil {
			return handled, err
		}
		event, decodeErr := DecodeProcessingEvent(message.Data)
		if decodeErr != nil {
			if termErr := message.Term(); termErr != nil {
				return handled, errors.Join(decodeErr, fmt.Errorf("terminate invalid cover message: %w", termErr))
			}
			continue
		}
		if event.EventID != message.Header.Get(nats.MsgIdHdr) {
			if termErr := message.Term(); termErr != nil {
				return handled, errors.Join(ErrInvalidCoverMessage, termErr)
			}
			continue
		}
		if handleErr := handler.Handle(ctx, event); handleErr != nil {
			if errors.Is(handleErr, ErrPermanentCoverProcessing) {
				if termErr := message.Term(); termErr != nil {
					return handled, errors.Join(handleErr, fmt.Errorf("terminate cover message: %w", termErr))
				}
				continue
			}
			metadata, metadataErr := message.Metadata()
			if metadataErr != nil {
				return handled, errors.Join(handleErr, fmt.Errorf("read cover delivery metadata: %w", metadataErr))
			}
			if int(metadata.NumDelivered) >= c.maxDeliver {
				if failureHandler, ok := handler.(CoverEventFailureHandler); ok {
					if failureErr := failureHandler.MarkFailed(ctx, event, handleErr); failureErr != nil {
						return handled, errors.Join(handleErr, fmt.Errorf("mark cover processing failed: %w", failureErr))
					}
				}
				if termErr := message.Term(); termErr != nil {
					return handled, errors.Join(handleErr, fmt.Errorf("terminate exhausted cover message: %w", termErr))
				}
				continue
			}
			if nakErr := message.NakWithDelay(c.retryDelay); nakErr != nil {
				return handled, errors.Join(handleErr, fmt.Errorf("retry cover message: %w", nakErr))
			}
			continue
		}
		if ackErr := message.AckSync(nats.Context(ctx)); ackErr != nil {
			return handled, fmt.Errorf("acknowledge cover message: %w", ackErr)
		}
		handled++
	}
	return handled, nil
}

func DecodeProcessingEvent(payload []byte) (ProcessingEvent, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var event ProcessingEvent
	if err := decoder.Decode(&event); err != nil {
		return ProcessingEvent{}, fmt.Errorf("%w: %v", ErrInvalidCoverMessage, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProcessingEvent{}, fmt.Errorf("%w: trailing JSON data", ErrInvalidCoverMessage)
	}
	if event.SchemaVersion != 1 || event.EventID == "" || event.CoverID == "" ||
		event.BookID < 1 || event.LibraryID == "" || event.SourceObjectKey == "" ||
		event.Attempt < 1 {
		return ProcessingEvent{}, ErrInvalidCoverMessage
	}
	return event, nil
}

func (c *JetStreamConsumer) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	if err := c.connection.Drain(); err != nil {
		c.connection.Close()
		return fmt.Errorf("drain NATS consumer connection: %w", err)
	}
	return nil
}
