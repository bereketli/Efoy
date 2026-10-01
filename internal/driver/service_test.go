package driver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/idgen"
	"github.com/blinge12/efoy/pkg/objstore"
)

type env struct {
	svc      *Service
	repo     *memRepo
	files    *fakeFiles
	roles    *fakeRoles
	vehicles fakeVehicles
	clock    *clock.Fake
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		repo:     newMemRepo(),
		files:    &fakeFiles{objects: map[string]objstore.Info{}},
		roles:    &fakeRoles{granted: map[uuid.UUID][]authz.Grant{}},
		vehicles: fakeVehicles{},
		clock:    clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)),
	}
	e.svc = NewService(e.repo, e.vehicles, e.files, e.roles, e.clock)
	return e
}

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

var reviewer = authz.Actor{UserID: idgen.New(), Grants: []authz.Grant{{Role: authz.RoleDispatcher, Scope: authz.ScopeGlobal}}}

func validInput() Input {
	return Input{Source: "TAXI", LicenceNumber: "aa-1234567", LicenceCategory: "Public 2", LicenceExpiresOn: date("2029-06-30")}
}

// registered returns an actor who is a registered driver, and the driver.
func (e *env) registered(t *testing.T) (authz.Actor, Driver) {
	t.Helper()
	actor := authz.Actor{UserID: idgen.New()}
	e.repo.names[actor.UserID] = "Abebe Kebede"
	in := validInput()
	in.LicenceNumber = "AA-" + actor.UserID.String()[24:] // UUIDv7 starts with the time; the tail is random
	d, err := e.svc.Register(context.Background(), actor, in)
	require.NoError(t, err)
	return actor, d
}

// upload simulates the client uploading a file and returns its key.
func (e *env) upload(t *testing.T, actor authz.Actor, ownerType string, ownerID uuid.UUID, docType string) string {
	t.Helper()
	up, err := e.svc.RequestUpload(context.Background(), actor, UploadRequest{
		OwnerType: ownerType, OwnerID: ownerID, DocType: docType, ContentType: "image/jpeg",
	})
	require.NoError(t, err)
	e.files.put(up.FileKey, "image/jpeg", 200_000)
	return up.FileKey
}

func (e *env) submit(t *testing.T, actor authz.Actor, d Driver) Document {
	t.Helper()
	key := e.upload(t, actor, OwnerDriver, d.ID, "DRIVING_LICENCE")
	doc, err := e.svc.Submit(context.Background(), actor, DocumentInput{
		OwnerType: OwnerDriver, OwnerID: d.ID, DocType: "DRIVING_LICENCE", FileKey: key,
		ExpiresOn: ptr(date("2029-06-30")),
	})
	require.NoError(t, err)
	return doc
}

func TestRegisterDriver(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	actor, d := e.registered(t)

	assert.Equal(t, StatusPending, d.Status)
	assert.Equal(t, "Abebe Kebede", d.FullName)
	assert.Equal(t, []authz.Grant{{Role: authz.RoleDriver, Scope: authz.ScopeGlobal}}, e.roles.granted[actor.UserID])
	assert.Equal(t, []string{"efoy.driver.registered"}, e.repo.subjects())

	_, err := e.svc.Register(ctx, actor, validInput())
	assert.ErrorIs(t, err, ErrDriverExists)

	other := authz.Actor{UserID: idgen.New()}
	in := validInput()
	in.LicenceNumber = d.LicenceNumber
	_, err = e.svc.Register(ctx, other, in)
	assert.ErrorIs(t, err, ErrLicenceTaken)
}

func TestRegisterDriverValidation(t *testing.T) {
	e := newEnv(t)
	actor := authz.Actor{UserID: idgen.New()}
	for name, tt := range map[string]struct {
		mutate func(*Input)
		want   error
	}{
		"bad source":        {func(in *Input) { in.Source = "UBER" }, ErrInvalidSource},
		"short licence":     {func(in *Input) { in.LicenceNumber = "A1" }, ErrInvalidLicence},
		"no category":       {func(in *Input) { in.LicenceCategory = " " }, ErrInvalidLicence},
		"expired licence":   {func(in *Input) { in.LicenceExpiresOn = date("2026-10-01") }, ErrLicenceExpired},
		"bad contact phone": {func(in *Input) { in.EmergencyContactPhone = ptr("0911000111") }, ErrInvalidContactInfo},
	} {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			_, err := e.svc.Register(context.Background(), actor, in)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUploadsAreLimitedToOwnDocuments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alice, aliceDriver := e.registered(t)
	bob, bobDriver := e.registered(t)
	bobsCar := idgen.New()
	e.vehicles[bobsCar] = bobDriver.ID

	req := UploadRequest{OwnerType: OwnerDriver, OwnerID: aliceDriver.ID, DocType: "DRIVING_LICENCE", ContentType: "application/pdf"}
	up, err := e.svc.RequestUpload(ctx, alice, req)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(up.FileKey, "documents/driver/"+aliceDriver.ID.String()+"/"))
	assert.True(t, strings.HasSuffix(up.FileKey, ".pdf"))
	assert.Equal(t, int64(MaxFileBytes), up.MaxBytes)

	_, err = e.svc.RequestUpload(ctx, bob, req)
	assert.ErrorIs(t, err, ErrNotYourDocument, "another driver's documents")

	vehicleReq := UploadRequest{OwnerType: OwnerVehicle, OwnerID: bobsCar, DocType: "INSURANCE", ContentType: "image/png"}
	_, err = e.svc.RequestUpload(ctx, alice, vehicleReq)
	assert.ErrorIs(t, err, ErrNotYourDocument, "another driver's vehicle")
	_, err = e.svc.RequestUpload(ctx, bob, vehicleReq)
	assert.NoError(t, err)

	_, err = e.svc.RequestUpload(ctx, authz.Actor{UserID: idgen.New()}, req)
	assert.ErrorIs(t, err, ErrNotYourDocument, "not a driver")

	bad := req
	bad.DocType = "INSURANCE"
	_, err = e.svc.RequestUpload(ctx, alice, bad)
	assert.ErrorIs(t, err, ErrInvalidDocType, "insurance belongs to vehicles")
	bad = req
	bad.ContentType = "image/gif"
	_, err = e.svc.RequestUpload(ctx, alice, bad)
	assert.ErrorIs(t, err, ErrUnsupportedFile)
}

func TestSubmitChecksTheUpload(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	actor, d := e.registered(t)
	key := e.upload(t, actor, OwnerDriver, d.ID, "DRIVING_LICENCE")
	in := DocumentInput{OwnerType: OwnerDriver, OwnerID: d.ID, DocType: "DRIVING_LICENCE", FileKey: key, ExpiresOn: ptr(date("2029-06-30"))}

	missing := in
	missing.FileKey = "documents/driver/" + d.ID.String() + "/never-uploaded.jpg"
	_, err := e.svc.Submit(ctx, actor, missing)
	assert.ErrorIs(t, err, ErrUploadMissing)

	foreign := in
	foreign.FileKey = "documents/driver/" + idgen.New().String() + "/x.jpg"
	_, err = e.svc.Submit(ctx, actor, foreign)
	assert.ErrorIs(t, err, ErrFileKeyMismatch)

	noExpiry := in
	noExpiry.ExpiresOn = nil
	_, err = e.svc.Submit(ctx, actor, noExpiry)
	assert.ErrorIs(t, err, ErrExpiryRequired)

	expired := in
	expired.ExpiresOn = ptr(date("2026-09-30"))
	_, err = e.svc.Submit(ctx, actor, expired)
	assert.ErrorIs(t, err, ErrExpired)

	e.files.put(key, "image/jpeg", MaxFileBytes+1)
	_, err = e.svc.Submit(ctx, actor, in)
	assert.ErrorIs(t, err, ErrFileTooLarge)

	e.files.put(key, "text/html", 100)
	_, err = e.svc.Submit(ctx, actor, in)
	assert.ErrorIs(t, err, ErrUnsupportedFile, "the stored type is checked, not the requested one")

	e.files.put(key, "image/jpeg; charset=binary", 100)
	doc, err := e.svc.Submit(ctx, actor, in)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, doc.Status)
	assert.Equal(t, "image/jpeg", doc.ContentType)
	assert.Contains(t, e.repo.subjects(), "efoy.document.submitted")

	_, err = e.svc.Submit(ctx, actor, in)
	assert.ErrorIs(t, err, ErrUploadReused)
}

func TestReviewDocuments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	actor, d := e.registered(t)
	doc := e.submit(t, actor, d)

	_, err := e.svc.Review(ctx, actor, doc.ID, "APPROVE", "")
	assert.ErrorIs(t, err, authz.ErrForbidden, "drivers cannot review")
	_, err = e.svc.Review(ctx, reviewer, doc.ID, "REJECT", "  ")
	assert.ErrorIs(t, err, ErrRejectNeedsReason)
	_, err = e.svc.Review(ctx, reviewer, doc.ID, "MAYBE", "")
	assert.ErrorIs(t, err, ErrInvalidDecision)
	_, err = e.svc.Review(ctx, reviewer, idgen.New(), "APPROVE", "")
	assert.ErrorIs(t, err, ErrDocumentNotFound)

	rejected, err := e.svc.Review(ctx, reviewer, doc.ID, "REJECT", "Blurry photo")
	require.NoError(t, err)
	assert.Equal(t, StatusRejected, rejected.Status)
	assert.Equal(t, ptr("Blurry photo"), rejected.RejectionReason)
	assert.Equal(t, &reviewer.UserID, rejected.ReviewedBy)
	require.NotNil(t, rejected.ViewURL)
	assert.Contains(t, e.repo.subjects(), "efoy.document.reviewed")

	_, err = e.svc.Review(ctx, reviewer, doc.ID, "APPROVE", "")
	assert.ErrorIs(t, err, ErrAlreadyReviewed)

	// The driver resubmits a better photo, which is approved.
	approved, err := e.svc.Review(ctx, reviewer, e.submit(t, actor, d).ID, "APPROVE", "ignored")
	require.NoError(t, err)
	assert.Equal(t, StatusApproved, approved.Status)
	assert.Nil(t, approved.RejectionReason)
}

func TestReviewQueuePagesOldestFirst(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	actor, d := e.registered(t)
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		ids = append(ids, e.submit(t, actor, d).ID)
		e.clock.Advance(time.Minute)
	}
	_, err := e.svc.Review(ctx, reviewer, ids[0], "APPROVE", "")
	require.NoError(t, err)

	_, err = e.svc.ReviewQueue(ctx, actor, nil, 2, "")
	assert.ErrorIs(t, err, authz.ErrForbidden)

	var seen []uuid.UUID
	cursor := ""
	for {
		page, err := e.svc.ReviewQueue(ctx, reviewer, nil, 2, cursor)
		require.NoError(t, err)
		for _, doc := range page.Items {
			seen = append(seen, doc.ID)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	assert.Equal(t, ids[1:], seen, "pending documents, oldest first, across pages")

	approved, err := e.svc.ReviewQueue(ctx, reviewer, []string{StatusApproved}, 0, "")
	require.NoError(t, err)
	assert.Len(t, approved.Items, 1)

	_, err = e.svc.ReviewQueue(ctx, reviewer, nil, 2, "not-a-cursor")
	assert.ErrorIs(t, err, ErrInvalidCursor)
	_, err = e.svc.ReviewQueue(ctx, reviewer, []string{"LOST"}, 2, "")
	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestProfileVisibility(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	actor, d := e.registered(t)
	car := idgen.New()
	e.vehicles[car] = d.ID
	e.submit(t, actor, d)

	mine, err := e.svc.MyProfile(ctx, actor)
	require.NoError(t, err)
	assert.Len(t, mine.Vehicles, 1)
	assert.Len(t, mine.Documents, 1)

	_, err = e.svc.ProfileByID(ctx, reviewer, d.ID)
	assert.NoError(t, err, "operations staff may view drivers")
	_, err = e.svc.ProfileByID(ctx, authz.Actor{UserID: idgen.New()}, d.ID)
	assert.ErrorIs(t, err, authz.ErrForbidden)
	_, err = e.svc.MyProfile(ctx, authz.Actor{UserID: idgen.New()})
	assert.ErrorIs(t, err, ErrDriverNotFound)
}
