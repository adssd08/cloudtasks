BEGIN;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id),

    title TEXT NOT NULL
        CHECK (char_length(btrim(title)) BETWEEN 1 AND 200),

    description TEXT NOT NULL DEFAULT ''
        CHECK (char_length(description) <= 5000),

    status TEXT NOT NULL DEFAULT 'OPEN'
        CHECK (status IN ('OPEN', 'IN_PROGRESS', 'COMPLETED')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tasks_owner_created_id_idx
    ON tasks (owner_id, created_at DESC, id DESC);

CREATE TABLE processed_jobs (
    consumer_name TEXT NOT NULL
        CHECK (char_length(btrim(consumer_name)) > 0),

    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (consumer_name, event_id)
);

COMMIT;