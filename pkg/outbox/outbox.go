// Package outbox implements the transactional outbox (design doc 7.2): domain
// events are written to outbox_events in the same transaction as the state
// change, and a relay publishes them to NATS afterwards. An event is therefore
// published if and only if its transaction commits.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/blinge12/efoy/pkg/idgen"
)

// Execer is satisfied by pgx.Tx. Events must be written inside the
// transaction of the change they describe.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Event is a domain event to publish on Subject (for example
// "efoy.document.submitted").
type Event struct {
	AggregateType string
	AggregateID   uuid.UUID
	Subject       string
	Data          any
	OccurredAt    time.Time
}

// Envelope is the JSON published to NATS. Consumers de-duplicate on EventID.
type Envelope struct {
	EventID    uuid.UUID       `json:"event_id"`
	Subject    string          `json:"subject"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// Add stores e in the outbox using db, which must be the caller's transaction.
func Add(ctx context.Context, db Execer, e Event) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return fmt.Errorf("outbox: marshal %s: %w", e.Subject, err)
	}
	id := idgen.New()
	payload, err := json.Marshal(Envelope{EventID: id, Subject: e.Subject, OccurredAt: e.OccurredAt.UTC(), Data: data})
	if err != nil {
		return fmt.Errorf("outbox: marshal envelope: %w", err)
	}
	headers, err := json.Marshal(map[string]string{"event_id": id.String()})
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, subject, payload, headers)
		VALUES ($1, $2, $3, $4::text::jsonb, $5::text::jsonb)`,
		e.AggregateType, e.AggregateID, e.Subject, string(payload), string(headers))
	if err != nil {
		return fmt.Errorf("outbox: insert %s: %w", e.Subject, err)
	}
	return nil
}
