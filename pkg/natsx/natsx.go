// Package natsx connects to NATS JetStream and declares the Efoy streams
// (design doc 12.9).
package natsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Streams. Subjects must not overlap between streams.
var (
	// DomainSubjects are durable business events, kept for 7 days.
	DomainSubjects = []string{
		"efoy.driver.>",
		"efoy.vehicle.>",
		"efoy.document.>",
		"efoy.subscription.>",
		"efoy.payment.>",
		"efoy.boarding.>",
		"efoy.incident.>",
		"efoy.trip.status_changed",
		"efoy.trip.stop_arrived",
		"efoy.trip.approaching",
	}
	// TrackingSubjects are high-volume GPS points, kept in memory for 5 minutes.
	TrackingSubjects = []string{"efoy.trip.*.location"}
)

// JetStream wraps a NATS connection with JetStream publishing.
type JetStream struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// Connect dials url, retrying in the background if NATS is not up yet.
func Connect(url, name string) (*JetStream, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("natsx: connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("natsx: jetstream: %w", err)
	}
	return &JetStream{nc: nc, js: js}, nil
}

// EnsureStreams creates or updates the DOMAIN and TRACKING streams.
func (j *JetStream) EnsureStreams(ctx context.Context, replicas int) error {
	if replicas < 1 {
		replicas = 1
	}
	streams := []jetstream.StreamConfig{
		{
			Name:       "DOMAIN",
			Subjects:   DomainSubjects,
			Storage:    jetstream.FileStorage,
			MaxAge:     7 * 24 * time.Hour,
			Replicas:   replicas,
			Duplicates: 2 * time.Minute,
		},
		{
			Name:     "TRACKING",
			Subjects: TrackingSubjects,
			Storage:  jetstream.MemoryStorage,
			MaxAge:   5 * time.Minute,
			Replicas: replicas,
		},
	}
	for _, cfg := range streams {
		if _, err := j.js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("natsx: stream %s: %w", cfg.Name, err)
		}
	}
	return nil
}

// Publish sends data and waits for the stream's acknowledgement. msgID makes
// retries within the duplicate window idempotent.
func (j *JetStream) Publish(ctx context.Context, subject string, data []byte, msgID string) error {
	_, err := j.js.Publish(ctx, subject, data, jetstream.WithMsgID(msgID))
	return err
}

// Ping reports whether the connection is up (readiness check).
func (j *JetStream) Ping(context.Context) error {
	if !j.nc.IsConnected() {
		return errors.New("natsx: not connected")
	}
	return nil
}

// JS exposes the JetStream context for consumers.
func (j *JetStream) JS() jetstream.JetStream { return j.js }

func (j *JetStream) Close() {
	_ = j.nc.Drain()
}
