CREATE TABLE board_roles (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id   bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name       text NOT NULL CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (board_id, name)
);

CREATE TABLE board_role_members (
    role_id    bigint NOT NULL REFERENCES board_roles (id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, user_id)
);

CREATE TABLE transitions (
    board_id       bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    from_column_id bigint NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
    to_column_id   bigint NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (from_column_id, to_column_id),
    CHECK (from_column_id <> to_column_id)
);

CREATE TABLE move_permissions (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id       bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    to_column_id   bigint NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
    from_column_id bigint REFERENCES columns (id) ON DELETE CASCADE,
    subject        text NOT NULL CHECK (subject IN ('any_member', 'assignee', 'team_lead', 'board_role')),
    board_role_id  bigint REFERENCES board_roles (id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CHECK ((subject = 'board_role') = (board_role_id IS NOT NULL))
);

CREATE INDEX move_permissions_board_idx ON move_permissions (board_id);

CREATE TABLE column_conditions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    column_id  bigint NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
    phase      text NOT NULL CHECK (phase IN ('enter', 'exit')),
    kind       text NOT NULL CHECK (kind IN ('has_assignee', 'has_estimate', 'has_due_date', 'has_description',
                                             'has_label', 'checklist_complete', 'blockers_done', 'min_attachments')),
    params     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX column_conditions_column_idx ON column_conditions (column_id);

-- Attachments arrive in phase 4; the table exists now so that the
-- min_attachments condition has something to count.
CREATE TABLE attachments (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id      bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    uploader_id  bigint NOT NULL REFERENCES users (id),
    filename     text NOT NULL,
    content_type text NOT NULL,
    size         bigint NOT NULL CHECK (size >= 0),
    storage_key  text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX attachments_card_idx ON attachments (card_id);
