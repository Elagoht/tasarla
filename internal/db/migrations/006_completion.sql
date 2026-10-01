-- A card is completed when it enters a done column: it leaves the board and is
-- listed among the board's done cards until it is reopened into a column.
ALTER TABLE cards
    ADD COLUMN completed_at timestamptz,
    -- The column a completed card came from, offered when it is reopened.
    ADD COLUMN completed_from_column_id bigint REFERENCES columns (id) ON DELETE SET NULL;

-- Cards already in a done column are done.
UPDATE cards k SET completed_at = now()
FROM columns c
WHERE c.id = k.column_id AND c.is_done AND k.completed_at IS NULL;

CREATE INDEX cards_completed_idx ON cards (board_id, completed_at DESC) WHERE completed_at IS NOT NULL;
