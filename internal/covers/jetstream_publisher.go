package covers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	DefaultCoverStreamMaxAge    = 72 * time.Hour
	DefaultCoverDuplicateWindow = 2 * time.Minute
)

type JetStreamPublisher struct {
	connection *nats.Conn
	jetStream  nats.JetStreamContext
	subject    string
}

func NewJetStreamPublisher(
	ctx context.Context,
	url string,
	user string,
	password string,
	streamName string,
	subject string,
) (*JetStreamPublisher, error) {
	if url == "" || streamName == "" || subject == "" {
		return nil, fmt.Errorf("invalid JetStream publisher configuration")
	}
	if (user == "") != (password == "") {
		return nil, fmt.Errorf("NATS user and password must be configured together")
	}

	options := []nats.Option{
		nats.Name("defta-cover-outbox-publisher"),
		nats.Timeout(5 * time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
	}
	if user != "" {
		options = append(options, nats.UserInfo(user, password))
	}
	connection, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("connect NATS: %w", err)
	}
	publisher := &JetStreamPublisher{connection: connection, subject: subject}
	publisher.jetStream, err = connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("open JetStream context: %w", err)
	}
	if err = publisher.ensureStream(ctx, streamName); err != nil {
		connection.Close()
		return nil, err
	}
	return publisher, nil
}

func (p *JetStreamPublisher) ensureStream(ctx context.Context, streamName string) error {
	info, err := p.jetStream.StreamInfo(streamName, nats.Context(ctx))
	if errors.Is(err, nats.ErrStreamNotFound) {
		_, err = p.jetStream.AddStream(&nats.StreamConfig{
			Name:            streamName,
			Subjects:        []string{p.subject},
			Retention:       nats.LimitsPolicy,
			Discard:         nats.DiscardOld,
			Storage:         nats.FileStorage,
			MaxAge:          DefaultCoverStreamMaxAge,
			Duplicates:      DefaultCoverDuplicateWindow,
			AllowDirect:     true,
			DenyDelete:      true,
			DenyPurge:       true,
		}, nats.Context(ctx))
		if err != nil {
			return fmt.Errorf("create cover JetStream stream: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect cover JetStream stream: %w", err)
	}
	if err = validateCoverStream(info.Config, p.subject); err != nil {
		return fmt.Errorf("incompatible cover JetStream stream: %w", err)
	}
	return nil
}

func validateCoverStream(config nats.StreamConfig, subject string) error {
	for _, configured := range config.Subjects {
		if configured == subject {
			if config.Storage != nats.FileStorage {
				return fmt.Errorf("stream storage must be file")
			}
			return nil
		}
	}
	return fmt.Errorf("stream does not contain subject %q", subject)
}

func (p *JetStreamPublisher) Publish(
	ctx context.Context,
	subject string,
	eventID string,
	payload []byte,
) error {
	if p == nil || p.jetStream == nil || eventID == "" || subject != p.subject {
		return fmt.Errorf("invalid cover JetStream message")
	}
	message := newCoverJetStreamMessage(subject, eventID, payload)
	ack, err := p.jetStream.PublishMsg(message, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish cover event: %w", err)
	}
	if ack == nil || ack.Stream == "" {
		return fmt.Errorf("publish cover event: missing JetStream acknowledgement")
	}
	return nil
}

func newCoverJetStreamMessage(subject, eventID string, payload []byte) *nats.Msg {
	message := nats.NewMsg(subject)
	message.Header.Set(nats.MsgIdHdr, eventID)
	message.Header.Set("Content-Type", "application/json")
	message.Data = append([]byte(nil), payload...)
	return message
}

func (p *JetStreamPublisher) Close() error {
	if p == nil || p.connection == nil {
		return nil
	}
	if err := p.connection.Drain(); err != nil {
		p.connection.Close()
		return fmt.Errorf("drain NATS connection: %w", err)
	}
	return nil
}
