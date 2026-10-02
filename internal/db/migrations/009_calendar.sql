-- A user's calendar feeds are reached with a token kept only as its SHA-256
-- (spec 2026-10-01 filtre… §5.1).
ALTER TABLE users ADD COLUMN calendar_token_hash bytea UNIQUE;
