CREATE TABLE boards (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    team_id          bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    name             text NOT NULL CHECK (btrim(name) <> ''),
    archived_at      timestamptz,
    transitions_mode text NOT NULL DEFAULT 'open' CHECK (transitions_mode IN ('open', 'restricted')),
    person_wip_limit int CHECK (person_wip_limit > 0),
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX boards_team_idx ON boards (team_id);

CREATE TABLE columns (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id          bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name              text NOT NULL CHECK (btrim(name) <> ''),
    position          int NOT NULL,
    wip_limit         int CHECK (wip_limit > 0),
    is_done           boolean NOT NULL DEFAULT false,
    allow_create      boolean NOT NULL DEFAULT false,
    counts_person_wip boolean NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX columns_board_idx ON columns (board_id, position);

CREATE TABLE cards (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id    bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    column_id   bigint NOT NULL REFERENCES columns (id),
    position    int NOT NULL,
    title       text NOT NULL CHECK (btrim(title) <> ''),
    description text NOT NULL DEFAULT '',
    assignee_id bigint REFERENCES users (id),
    estimate    numeric CHECK (estimate >= 0),
    due_date    date,
    priority    smallint CHECK (priority BETWEEN 1 AND 4),
    created_by  bigint NOT NULL REFERENCES users (id),
    version     int NOT NULL DEFAULT 1,
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cards_column_idx ON cards (column_id, position) WHERE archived_at IS NULL;
CREATE INDEX cards_board_idx ON cards (board_id);
CREATE INDEX cards_assignee_idx ON cards (assignee_id) WHERE archived_at IS NULL;

CREATE TABLE labels (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id   bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name       text NOT NULL CHECK (btrim(name) <> ''),
    color      text NOT NULL CHECK (color ~ '^#[0-9a-f]{6}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (board_id, name)
);

CREATE TABLE card_labels (
    card_id    bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    label_id   bigint NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (card_id, label_id)
);

CREATE TABLE checklist_items (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id    bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    text       text NOT NULL CHECK (btrim(text) <> ''),
    done       boolean NOT NULL DEFAULT false,
    position   int NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX checklist_items_card_idx ON checklist_items (card_id, position);

CREATE TABLE card_dependencies (
    blocker_id bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    blocked_id bigint NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);

CREATE INDEX card_dependencies_blocked_idx ON card_dependencies (blocked_id);
