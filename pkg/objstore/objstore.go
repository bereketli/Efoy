// Package objstore stores files in S3-compatible storage (MinIO in the design
// doc 8.2; RustFS locally and on staging): driver
// documents, photos and exports. Clients upload and download directly with
// short-lived pre-signed URLs; the API only signs and checks.
package objstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrNotFound means no object exists at the key.
var ErrNotFound = errors.New("objstore: object not found")

// Config describes one bucket.
type Config struct {
	Endpoint  string // host:port reachable from the services
	AccessKey string
	SecretKey string
	UseSSL    bool
	Region    string
	PublicURL string // base URL reachable from apps and browsers
	Bucket    string
}

// Info describes a stored object.
type Info struct {
	Size        int64
	ContentType string
}

// Store signs URLs against the public endpoint and reads objects through the
// internal one. Signing is local; setting the region avoids a network lookup.
type Store struct {
	internal *minio.Client
	public   *minio.Client
	bucket   string
}

func New(cfg Config) (*Store, error) {
	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	internal, err := minio.New(cfg.Endpoint, &minio.Options{Creds: creds, Secure: cfg.UseSSL, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("objstore: %w", err)
	}
	pub, err := url.Parse(cfg.PublicURL)
	if err != nil || pub.Host == "" {
		return nil, fmt.Errorf("objstore: invalid public url %q", cfg.PublicURL)
	}
	public, err := minio.New(pub.Host, &minio.Options{Creds: creds, Secure: pub.Scheme == "https", Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("objstore: %w", err)
	}
	return &Store{internal: internal, public: public, bucket: cfg.Bucket}, nil
}

// PresignPut returns a URL the client can PUT the file to until ttl passes.
func (s *Store) PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.public.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return "", fmt.Errorf("objstore: presign put: %w", err)
	}
	return u.String(), nil
}

// PresignGet returns a URL to download the file until ttl passes.
func (s *Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.public.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("objstore: presign get: %w", err)
	}
	return u.String(), nil
}

// Stat returns the object's size and content type.
func (s *Store) Stat(ctx context.Context, key string) (Info, error) {
	oi, err := s.internal.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return Info{}, ErrNotFound
		}
		return Info{}, fmt.Errorf("objstore: stat: %w", err)
	}
	return Info{Size: oi.Size, ContentType: oi.ContentType}, nil
}

// SHA256 streams the object and returns its digest. Callers check the size
// with Stat first.
func (s *Store) SHA256(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.internal.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("objstore: get: %w", err)
	}
	defer func() { _ = obj.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, obj); err != nil {
		return nil, fmt.Errorf("objstore: read: %w", err)
	}
	return h.Sum(nil), nil
}

// EnsureBucket creates the bucket if it does not exist yet.
func (s *Store) EnsureBucket(ctx context.Context) error {
	ok, err := s.internal.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("objstore: bucket %s: %w", s.bucket, err)
	}
	if ok {
		return nil
	}
	if err := s.internal.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		if exists, err2 := s.internal.BucketExists(ctx, s.bucket); err2 == nil && exists {
			return nil // created concurrently by another replica
		}
		return fmt.Errorf("objstore: create bucket %s: %w", s.bucket, err)
	}
	return nil
}

// Ping checks that the bucket exists (readiness check).
func (s *Store) Ping(ctx context.Context) error {
	ok, err := s.internal.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("objstore: bucket %s does not exist", s.bucket)
	}
	return nil
}
