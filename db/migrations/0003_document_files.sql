-- +goose Up
-- File metadata recorded when an upload is submitted, so reviewers know how
-- to display it; each uploaded object backs at most one document.
ALTER TABLE documents
  ADD COLUMN content_type text NOT NULL DEFAULT 'application/octet-stream',
  ADD COLUMN size_bytes   bigint CHECK (size_bytes > 0);

CREATE UNIQUE INDEX uq_documents_file_key ON documents (file_key);

-- +goose Down
DROP INDEX uq_documents_file_key;
ALTER TABLE documents
  DROP COLUMN size_bytes,
  DROP COLUMN content_type;
