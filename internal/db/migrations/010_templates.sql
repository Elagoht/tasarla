-- Card templates, their schedules and the runs a schedule made (spec
-- 2026-10-01 filtre… §6).
CREATE TABLE card_templates (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    board_id          bigint NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    name              text NOT NULL CHECK (btrim(name) <> ''),
    title             text NOT NULL CHECK (btrim(title) <> ''),
    description       text NOT NULL DEFAULT '',
    priority          smallint CHECK (priority BETWEEN 1 AND 4),
    estimate          numeric CHECK (estimate >= 0),
    assignee_id       bigint REFERENCES users (id) ON DELETE SET NULL,
    column_id         bigint REFERENCES columns (id) ON DELETE SET NULL,
    due_in_days       int CHECK (due_in_days BETWEEN 0 AND 365),
    schedule_kind     text NOT NULL DEFAULT '' CHECK (schedule_kind IN ('', 'daily', 'weekly', 'monthly')),
    schedule_weekdays smallint NOT NULL DEFAULT 0 CHECK (schedule_weekdays BETWEEN 0 AND 127),
    schedule_monthday smallint NOT NULL DEFAULT 1 CHECK (schedule_monthday BETWEEN 1 AND 31),
    schedule_time     time NOT NULL DEFAULT '09:00',
    schedule_since    timestamptz,
    updated_by        bigint NOT NULL REFERENCES users (id),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (board_id, name),
    CHECK (schedule_kind <> 'weekly' OR schedule_weekdays > 0)
);

CREATE TABLE card_template_labels (
    template_id bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    label_id    bigint NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
    PRIMARY KEY (template_id, label_id)
);

CREATE TABLE card_template_checklist (
    template_id bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    position    int NOT NULL,
    text        text NOT NULL CHECK (btrim(text) <> ''),
    PRIMARY KEY (template_id, position)
);

CREATE TABLE template_runs (
    template_id   bigint NOT NULL REFERENCES card_templates (id) ON DELETE CASCADE,
    scheduled_for timestamptz NOT NULL,
    status        text NOT NULL CHECK (status IN ('created', 'failed')),
    card_id       bigint REFERENCES cards (id) ON DELETE SET NULL,
    violations    jsonb NOT NULL DEFAULT '[]',
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (template_id, scheduled_for)
);

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked', 'template_failed'));
ALTER TABLE notification_prefs DROP CONSTRAINT notification_prefs_kind_check;
ALTER TABLE notification_prefs ADD CONSTRAINT notification_prefs_kind_check
    CHECK (kind IN ('assigned', 'mentioned', 'commented', 'due_soon', 'overdue', 'unblocked', 'template_failed'));
