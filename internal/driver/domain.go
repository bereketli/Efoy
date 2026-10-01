// Package driver handles driver registration and the documents of drivers and
// their vehicles, including the admin review workflow (FR-DRV-1, FR-DRV-3).
package driver

import (
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/errs"
)

type Driver struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	FullName              string  // from users, read-only here
	Phone                 *string // from users, read-only here
	Source                string  // TAXI | YANGO | INDEPENDENT
	LicenceNumber         string
	LicenceCategory       string
	LicenceExpiresOn      time.Time
	EmergencyContactName  *string
	EmergencyContactPhone *string
	Status                string
	StudentCertified      bool
	CreatedAt             time.Time
}

// Input is a driver registration request.
type Input struct {
	Source                string
	LicenceNumber         string
	LicenceCategory       string
	LicenceExpiresOn      time.Time
	EmergencyContactName  *string
	EmergencyContactPhone *string
}

const (
	OwnerDriver  = "DRIVER"
	OwnerVehicle = "VEHICLE"

	StatusPending     = "PENDING"
	StatusUnderReview = "UNDER_REVIEW"
	StatusApproved    = "APPROVED"
	StatusRejected    = "REJECTED"
	StatusExpired     = "EXPIRED"
)

var (
	sources = []string{"TAXI", "YANGO", "INDEPENDENT"}

	// Document types accepted per owner (design doc 4.1).
	driverDocTypes  = []string{"DRIVING_LICENCE", "POLICE_CLEARANCE", "PHOTO", "YANGO_PROFILE", "OTHER"}
	vehicleDocTypes = []string{"VEHICLE_LIBRE", "INSURANCE", "ANNUAL_INSPECTION", "PHOTO", "OTHER"}

	// These expire and drive the reminders and suspension of FR-DRV-5.
	expiringDocTypes = []string{"DRIVING_LICENCE", "INSURANCE", "ANNUAL_INSPECTION"}

	// Accepted uploads and the file extension used in the object key.
	contentTypes = map[string]string{
		"image/jpeg":      ".jpg",
		"image/png":       ".png",
		"image/webp":      ".webp",
		"application/pdf": ".pdf",
	}

	documentStatuses = []string{StatusPending, StatusUnderReview, StatusApproved, StatusRejected, StatusExpired}

	phoneRE = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// MaxFileBytes is the largest document upload.
const MaxFileBytes = 10 << 20

func docTypeAllowed(ownerType, docType string) bool {
	switch ownerType {
	case OwnerDriver:
		return slices.Contains(driverDocTypes, docType)
	case OwnerVehicle:
		return slices.Contains(vehicleDocTypes, docType)
	}
	return false
}

// Document is a row of documents, plus a label for its owner.
type Document struct {
	ID              uuid.UUID
	OwnerType       string
	OwnerID         uuid.UUID
	OwnerLabel      string // driver name or vehicle plate
	DocType         string
	FileKey         string
	SHA256          []byte
	ContentType     string
	SizeBytes       int64
	DocNumber       *string
	IssuedOn        *time.Time
	ExpiresOn       *time.Time
	Status          string
	ReviewedBy      *uuid.UUID
	ReviewedAt      *time.Time
	RejectionReason *string
	CreatedAt       time.Time
	ViewURL         *string // admin views only
}

// Cursor positions the review queue (created_at, id), oldest first.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Repository sentinel errors.
var (
	ErrNotFound        = errors.New("driver: not found")
	ErrDuplicateUser   = errors.New("driver: user already registered")
	ErrDuplicate       = errors.New("driver: licence already registered")
	ErrDuplicateUpload = errors.New("driver: file already submitted")
)

// Client-facing errors.
var (
	ErrDriverExists       = errs.Conflict("DRIVER_EXISTS", "You are already registered as a driver.")
	ErrLicenceTaken       = errs.Conflict("LICENCE_TAKEN", "A driver with this licence number is already registered.")
	ErrDriverNotFound     = errs.NotFound("DRIVER_NOT_FOUND", "No driver profile was found.")
	ErrDocumentNotFound   = errs.NotFound("DOCUMENT_NOT_FOUND", "No such document.")
	ErrVehicleNotFound    = errs.NotFound("VEHICLE_NOT_FOUND", "No such vehicle.")
	ErrNotYourDocument    = errs.Forbidden("NOT_DOCUMENT_OWNER", "You can only upload documents for yourself and your own vehicles.")
	ErrUploadMissing      = errs.Invalid("UPLOAD_MISSING", "Upload the file to upload_url before submitting it.")
	ErrFileTooLarge       = errs.Invalid("FILE_TOO_LARGE", "Files can be at most 10 MB.")
	ErrUnsupportedFile    = errs.Invalid("UNSUPPORTED_FILE_TYPE", "Upload a JPEG, PNG, WebP or PDF file.")
	ErrFileKeyMismatch    = errs.Invalid("FILE_KEY_MISMATCH", "file_key does not belong to this owner.")
	ErrUploadReused       = errs.Conflict("DOCUMENT_EXISTS", "This file has already been submitted.")
	ErrAlreadyReviewed    = errs.Conflict("DOCUMENT_ALREADY_REVIEWED", "This document has already been reviewed.")
	ErrRejectNeedsReason  = errs.Invalid("REASON_REQUIRED", "Give the driver a reason when rejecting a document.")
	ErrInvalidDecision    = errs.Invalid("INVALID_DECISION", "decision must be APPROVE or REJECT.")
	ErrInvalidCursor      = errs.Invalid("INVALID_CURSOR", "cursor is not valid; start again from the first page.")
	ErrInvalidStatus      = errs.Invalid("INVALID_STATUS", "status must be PENDING, UNDER_REVIEW, APPROVED, REJECTED or EXPIRED.")
	ErrInvalidOwnerType   = errs.Invalid("INVALID_OWNER_TYPE", "owner_type must be DRIVER or VEHICLE.")
	ErrInvalidDocType     = errs.Invalid("INVALID_DOC_TYPE", "doc_type is not accepted for this owner_type.")
	ErrExpiryRequired     = errs.Invalid("EXPIRY_REQUIRED", "expires_on is required for this document type.")
	ErrExpired            = errs.Invalid("DOCUMENT_EXPIRED", "The document has already expired.")
	ErrIssuedInFuture     = errs.Invalid("INVALID_ISSUED_ON", "issued_on cannot be in the future or after expires_on.")
	ErrInvalidSource      = errs.Invalid("INVALID_SOURCE", "source must be TAXI, YANGO or INDEPENDENT.")
	ErrInvalidLicence     = errs.Invalid("INVALID_LICENCE", "licence_number (3–40 characters) and licence_category are required.")
	ErrLicenceExpired     = errs.Invalid("LICENCE_EXPIRED", "licence_expires_on must be in the future.")
	ErrInvalidContactInfo = errs.Invalid("INVALID_EMERGENCY_CONTACT", "emergency_contact_phone must be in E.164 format and the name at most 120 characters.")
)
