CREATE TABLE comments (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id    bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    author_id  bigint NOT NULL REFERENCES users (id),
    body       text NOT NULL CHECK (btrim(body) <> ''),
    edited_at  timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX comments_card_idx ON comments (card_id, id);

CREATE TABLE comment_mentions (
    comment_id bigint NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (comment_id, user_id)
);

CREATE TABLE activity (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id   bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    card_id    bigint REFERENCES cards (id) ON DELETE CASCADE,
    actor_id   bigint REFERENCES users (id),
    kind       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX activity_board_idx ON activity (board_id, id DESC);
CREATE INDEX activity_card_idx ON activity (card_id, id DESC);
