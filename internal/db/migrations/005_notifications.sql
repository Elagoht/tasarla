CREATE TABLE notifications (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked')),
    card_id    bigint REFERENCES cards (id) ON DELETE CASCADE,
    payload    jsonb NOT NULL DEFAULT '{}',
    read_at    timestamptz,
    dedupe_key text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_idx ON notifications (user_id, id DESC);
CREATE INDEX notifications_unread_idx ON notifications (user_id) WHERE read_at IS NULL;

CREATE TABLE notification_prefs (
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked')),
    email      boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, kind)
);

CREATE TABLE email_outbox (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    to_address      text NOT NULL,
    subject         text NOT NULL,
    html            text NOT NULL,
    text            text NOT NULL,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX email_outbox_pending_idx ON email_outbox (next_attempt_at) WHERE status = 'pending';
