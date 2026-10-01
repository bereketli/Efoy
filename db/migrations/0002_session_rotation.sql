-- +goose Up
-- Refresh-token rotation needs to tell a rotated token (a benign race between
-- two parallel refreshes) from a logged-out or stolen one, and staff portal
-- sessions expire 12 h after login instead of sliding (design doc 13.2).
ALTER TABLE sessions
  ADD COLUMN revoked_reason text CHECK (revoked_reason IN ('ROTATED','LOGOUT','REUSE_DETECTED','ADMIN')),
  ADD COLUMN sliding        boolean NOT NULL DEFAULT true,
  ADD CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL));

CREATE INDEX ix_sessions_family ON sessions (family_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX ix_sessions_family;
ALTER TABLE sessions
  DROP COLUMN sliding,
  DROP COLUMN revoked_reason;
