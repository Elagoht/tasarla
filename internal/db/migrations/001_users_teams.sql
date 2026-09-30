CREATE TABLE users (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    issuer      text NOT NULL,
    subject     text NOT NULL,
    email       text NOT NULL,
    name        text NOT NULL,
    locale      text NOT NULL CHECK (locale IN ('tr', 'en')),
    is_admin    boolean NOT NULL DEFAULT false,
    disabled_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE INDEX users_email_idx ON users (lower(email));

CREATE TABLE teams (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL CHECK (btrim(name) <> ''),
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE team_members (
    team_id    bigint NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('lead', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX team_members_user_idx ON team_members (user_id);
