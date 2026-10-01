package driver

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/internal/vehicle"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/objstore"
	"github.com/blinge12/efoy/pkg/outbox"
)

type memRepo struct {
	mu      sync.Mutex
	drivers map[uuid.UUID]Driver
	docs    map[uuid.UUID]Document
	events  []outbox.Event
	names   map[uuid.UUID]string // user id -> full name
}

func newMemRepo() *memRepo {
	return &memRepo{drivers: map[uuid.UUID]Driver{}, docs: map[uuid.UUID]Document{}, names: map[uuid.UUID]string{}}
}

func (m *memRepo) WithTx(_ context.Context, fn func(Repo) error) error { return fn(m) }

func (m *memRepo) Create(_ context.Context, d Driver) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.drivers {
		if o.UserID == d.UserID {
			return ErrDuplicateUser
		}
		if o.LicenceNumber == d.LicenceNumber {
			return ErrDuplicate
		}
	}
	m.drivers[d.ID] = d
	return nil
}

func (m *memRepo) withName(d Driver) Driver {
	d.FullName = m.names[d.UserID]
	return d
}

func (m *memRepo) Get(_ context.Context, id uuid.UUID) (Driver, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drivers[id]
	if !ok {
		return Driver{}, ErrNotFound
	}
	return m.withName(d), nil
}

func (m *memRepo) GetByUser(_ context.Context, userID uuid.UUID) (Driver, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.drivers {
		if d.UserID == userID {
			return m.withName(d), nil
		}
	}
	return Driver{}, ErrNotFound
}

func (m *memRepo) CreateDocument(_ context.Context, d Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.docs {
		if o.FileKey == d.FileKey {
			return ErrDuplicateUpload
		}
	}
	m.docs[d.ID] = d
	return nil
}

func (m *memRepo) GetDocument(_ context.Context, id uuid.UUID) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.docs[id]
	if !ok {
		return Document{}, ErrNotFound
	}
	return d, nil
}

func (m *memRepo) GetDocumentForUpdate(ctx context.Context, id uuid.UUID) (Document, error) {
	return m.GetDocument(ctx, id)
}

func (m *memRepo) SetDocumentReview(_ context.Context, id uuid.UUID, status string, reviewer uuid.UUID, at time.Time, reason *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.docs[id]
	d.Status, d.ReviewedBy, d.ReviewedAt, d.RejectionReason = status, &reviewer, &at, reason
	m.docs[id] = d
	return nil
}

func (m *memRepo) sorted(keep func(Document) bool) []Document {
	out := []Document{}
	for _, d := range m.docs {
		if keep(d) {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b Document) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), slices.Compare(a.ID[:], b.ID[:]))
	})
	return out
}

func (m *memRepo) ListDocumentsByStatus(_ context.Context, statuses []string, after Cursor, limit int) ([]Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.sorted(func(d Document) bool {
		past := d.CreatedAt.After(after.CreatedAt) ||
			(d.CreatedAt.Equal(after.CreatedAt) && slices.Compare(d.ID[:], after.ID[:]) > 0)
		return slices.Contains(statuses, d.Status) && past
	})
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
}

func (m *memRepo) ListDocumentsByOwners(_ context.Context, ownerIDs []uuid.UUID) ([]Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	docs := m.sorted(func(d Document) bool { return slices.Contains(ownerIDs, d.OwnerID) })
	slices.Reverse(docs)
	return docs, nil
}

func (m *memRepo) AddEvent(_ context.Context, e outbox.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

func (m *memRepo) subjects() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for _, e := range m.events {
		out = append(out, e.Subject)
	}
	return out
}

// fakeVehicles maps vehicle id -> owning driver id.
type fakeVehicles map[uuid.UUID]uuid.UUID

func (f fakeVehicles) OwnedByDriver(_ context.Context, driverID uuid.UUID) ([]vehicle.Vehicle, error) {
	out := []vehicle.Vehicle{}
	for id, owner := range f {
		if owner == driverID {
			o := owner
			out = append(out, vehicle.Vehicle{ID: id, OwnerDriverID: &o})
		}
	}
	return out, nil
}

func (f fakeVehicles) OwnerDriverID(_ context.Context, vehicleID uuid.UUID) (*uuid.UUID, error) {
	owner, ok := f[vehicleID]
	if !ok {
		return nil, vehicle.ErrNotFound
	}
	return &owner, nil
}

// fakeFiles is an in-memory object store.
type fakeFiles struct {
	mu      sync.Mutex
	objects map[string]objstore.Info
}

func (f *fakeFiles) put(key, contentType string, size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = objstore.Info{Size: size, ContentType: contentType}
}

func (f *fakeFiles) PresignPut(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://files.test/upload/" + key, nil
}

func (f *fakeFiles) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "http://files.test/view/" + key, nil
}

func (f *fakeFiles) Stat(_ context.Context, key string) (objstore.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info, ok := f.objects[key]
	if !ok {
		return objstore.Info{}, objstore.ErrNotFound
	}
	return info, nil
}

func (f *fakeFiles) SHA256(context.Context, string) ([]byte, error) { return make([]byte, 32), nil }

type fakeRoles struct {
	mu      sync.Mutex
	granted map[uuid.UUID][]authz.Grant
}

func (f *fakeRoles) GrantRole(_ context.Context, userID uuid.UUID, g authz.Grant) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !slices.Contains(f.granted[userID], g) {
		f.granted[userID] = append(f.granted[userID], g)
	}
	return nil
}
