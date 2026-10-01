-- Searching cards by title, description and comments, ignoring case the way
-- Turkish needs (spec 2026-10-01 filtre-swimlane… §3.3).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- kanban_fold is the text a search compares: lower case, the dotless ı as i,
-- and the combining dot some C libraries leave after lowering İ dropped, so
-- that ı, I, i and İ all match one another.
CREATE FUNCTION kanban_fold(t text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN translate(lower(t), 'ı' || chr(775), 'i');

CREATE INDEX cards_title_fold_trgm ON cards USING gin (kanban_fold(title) gin_trgm_ops);
CREATE INDEX cards_description_fold_trgm ON cards USING gin (kanban_fold(description) gin_trgm_ops);
CREATE INDEX comments_body_fold_trgm ON comments USING gin (kanban_fold(body) gin_trgm_ops);
