package services

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"

	_ "github.com/mattn/go-sqlite3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
)

func TestCoverUploadOutboxJetStreamIntegration(t *testing.T) {
	if os.Getenv("COVER_PIPELINE_INTEGRATION") != "1" {
		t.Skip("set COVER_PIPELINE_INTEGRATION=1 to test MinIO and JetStream")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open(
		"sqlite3",
		filepath.Join(t.TempDir(), "cover-pipeline.db")+"?_foreign_keys=on",
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err = db.ExecContext(ctx, `
		INSERT INTO users(
			id, username, password_hash, role, status, created_at, updated_at
		) VALUES (
			'owner-pipeline', 'owner-pipeline', 'hash',
			'OWNER_LIBRARY', 'ACTIVE', 'now', 'now'
		);
		INSERT INTO libraries(
			id, name, owner_user_id, status, created_at, updated_at
		) VALUES (
			'library-pipeline', 'Pipeline', 'owner-pipeline',
			'ACTIVE', 'now', 'now'
		);
	`); err != nil {
		t.Fatalf("seed identities: %v", err)
	}

	claims := &auth.Claims{
		Role:      models.RoleOwnerLibrary,
		LibraryID: "library-pipeline",
	}
	claims.Subject = "owner-pipeline"
	bookService := NewBookService(repositories.NewBookRepository(db))
	book, err := bookService.Create(ctx, claims, models.BookInput{
		Title:  "Livre pipeline couverture",
		Price:  1000,
		Volume: 1,
	})
	if err != nil {
		t.Fatalf("create book: %v", err)
	}

	minioStore, err := covers.NewMinIOStore(
		integrationEnv("MINIO_ENDPOINT", "127.0.0.1:9000"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		integrationEnv("MINIO_BUCKET_COVERS", "book-covers"),
		false,
	)
	if err != nil {
		t.Fatalf("new MinIO store: %v", err)
	}
	uploader, err := covers.NewSourceUploader(
		minioStore,
		covers.NewValidator(1024*1024, 100),
		identity.NewID,
	)
	if err != nil {
		t.Fatalf("new source uploader: %v", err)
	}
	coverService := NewBookCoverService(
		true,
		bookService,
		repositories.NewCoverRepository(db),
		uploader,
	)
	pending, err := coverService.Upload(
		ctx,
		claims,
		book.ID,
		"image/png",
		bytes.NewReader(coverPNG(t)),
	)
	if err != nil {
		t.Fatalf("upload cover: %v", err)
	}

	var eventID, sourceKey string
	if err = db.QueryRowContext(ctx, `
		SELECT event_id, source_object_key
		FROM cover_processing_outbox
		JOIN book_covers ON book_covers.id = cover_processing_outbox.cover_id
		WHERE cover_processing_outbox.cover_id = ?
	`, pending.ID).Scan(&eventID, &sourceKey); err != nil {
		t.Fatalf("read pending outbox: %v", err)
	}
	t.Cleanup(func() { _ = minioStore.Delete(context.Background(), sourceKey) })

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	streamName := "COVER_PIPELINE_" + suffix
	subject := "book.covers.pipeline." + suffix
	natsURL := integrationEnv("NATS_URL", nats.DefaultURL)
	natsUser := os.Getenv("NATS_USER")
	natsPassword := os.Getenv("NATS_PASSWORD")

	transport, err := covers.NewJetStreamPublisher(
		ctx, natsURL, natsUser, natsPassword, streamName, subject,
	)
	if err != nil {
		t.Fatalf("new JetStream publisher: %v", err)
	}
	t.Cleanup(func() { _ = transport.Close() })

	options := []nats.Option{}
	if natsUser != "" {
		options = append(options, nats.UserInfo(natsUser, natsPassword))
	}
	inspectionConnection, err := nats.Connect(natsURL, options...)
	if err != nil {
		t.Fatalf("connect inspection client: %v", err)
	}
	t.Cleanup(inspectionConnection.Close)
	inspection, err := inspectionConnection.JetStream()
	if err != nil {
		t.Fatalf("open inspection JetStream: %v", err)
	}
	t.Cleanup(func() { _ = inspection.DeleteStream(streamName) })

	outboxPublisher, err := NewCoverOutboxPublisher(
		repositories.NewCoverOutboxRepository(db),
		transport,
		"pipeline-publisher",
		subject,
	)
	if err != nil {
		t.Fatalf("new outbox publisher: %v", err)
	}
	count, err := outboxPublisher.PublishAvailable(ctx, 10)
	if err != nil {
		t.Fatalf("publish outbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("published count=%d", count)
	}

	var publishedAt sql.NullString
	var attempts int
	var lockedBy sql.NullString
	if err = db.QueryRowContext(ctx, `
		SELECT published_at, attempts, locked_by
		FROM cover_processing_outbox
		WHERE event_id = ?
	`, eventID).Scan(&publishedAt, &attempts, &lockedBy); err != nil {
		t.Fatalf("read published outbox: %v", err)
	}
	if !publishedAt.Valid || attempts != 0 || lockedBy.Valid {
		t.Fatalf(
			"published=%v attempts=%d locked=%v",
			publishedAt,
			attempts,
			lockedBy,
		)
	}

	message, err := inspection.GetMsg(streamName, 1, nats.Context(ctx))
	if err != nil {
		t.Fatalf("read JetStream message: %v", err)
	}
	if message.Header.Get(nats.MsgIdHdr) != eventID {
		t.Fatalf(
			"message id=%q want=%q",
			message.Header.Get(nats.MsgIdHdr),
			eventID,
		)
	}
	if len(message.Data) == 0 {
		t.Fatal("published payload is empty")
	}

	processingStore, err := covers.NewMinIOProcessingStore(
		integrationEnv("MINIO_ENDPOINT", "127.0.0.1:9000"),
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		integrationEnv("MINIO_BUCKET_COVERS", "book-covers"),
		false,
	)
	if err != nil {
		t.Fatalf("new processing store: %v", err)
	}
	variantProcessor, err := NewStoredCoverVariantProcessor(
		processingStore,
		processingStore,
		covers.NewImageProcessor(),
	)
	if err != nil {
		t.Fatalf("new variant processor: %v", err)
	}
	worker, err := NewBookCoverWorker(
		repositories.NewCoverProcessingRepository(db),
		variantProcessor,
		"pipeline-worker",
	)
	if err != nil {
		t.Fatalf("new cover worker: %v", err)
	}
	consumer, err := covers.NewJetStreamConsumer(
		ctx,
		natsURL,
		natsUser,
		natsPassword,
		streamName,
		subject,
		"pipeline-worker-"+suffix,
		3,
	)
	if err != nil {
		t.Fatalf("new JetStream consumer: %v", err)
	}
	t.Cleanup(func() { _ = consumer.Close() })

	handled, err := consumer.FetchAndHandle(ctx, 1, 10*time.Second, worker)
	if err != nil {
		t.Fatalf("consume and process cover: %v", err)
	}
	if handled != 1 {
		t.Fatalf("handled count=%d", handled)
	}

	var (
		status                                   string
		active                                   int
		masterKey, largeJPEGKey, largeWebPKey   string
		thumbJPEGKey, thumbWebPKey               string
	)
	if err = db.QueryRowContext(ctx, `
		SELECT status, active, master_object_key,
		       large_jpeg_object_key, large_webp_object_key,
		       thumb_jpeg_object_key, thumb_webp_object_key
		FROM book_covers
		WHERE id = ?
	`, pending.ID).Scan(
		&status,
		&active,
		&masterKey,
		&largeJPEGKey,
		&largeWebPKey,
		&thumbJPEGKey,
		&thumbWebPKey,
	); err != nil {
		t.Fatalf("read processed cover: %v", err)
	}
	if status != "READY" || active != 1 {
		t.Fatalf("status=%q active=%d", status, active)
	}

	minioClient, err := minio.New(
		integrationEnv("MINIO_ENDPOINT", "127.0.0.1:9000"),
		&minio.Options{
			Creds: credentials.NewStaticV4(
				os.Getenv("MINIO_ACCESS_KEY"),
				os.Getenv("MINIO_SECRET_KEY"),
				"",
			),
			Secure: false,
		},
	)
	if err != nil {
		t.Fatalf("new MinIO inspection client: %v", err)
	}
	bucket := integrationEnv("MINIO_BUCKET_COVERS", "book-covers")
	generatedKeys := []string{
		masterKey,
		largeJPEGKey,
		largeWebPKey,
		thumbJPEGKey,
		thumbWebPKey,
	}
	for _, key := range generatedKeys {
		if key == "" {
			t.Fatal("processed cover contains an empty object key")
		}
		if _, err = minioClient.StatObject(
			ctx,
			bucket,
			key,
			minio.StatObjectOptions{},
		); err != nil {
			t.Fatalf("stat generated object %q: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for _, key := range generatedKeys {
			_ = processingStore.DeleteVariant(context.Background(), key)
		}
	})
}

func integrationEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func TestCoverPipelineIntegrationConfigurationRequiresCredentials(t *testing.T) {
	if os.Getenv("COVER_PIPELINE_INTEGRATION") == "1" {
		if os.Getenv("MINIO_ACCESS_KEY") == "" ||
			os.Getenv("MINIO_SECRET_KEY") == "" {
			t.Fatal(fmt.Errorf("MinIO integration credentials are required"))
		}
	}
}
