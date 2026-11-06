BEGIN;

CREATE TABLE attachments (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    object_key TEXT NOT NULL UNIQUE,

    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'READY')),

    size_bytes BIGINT,
    content_type TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ,

    CHECK (
        (
            status = 'PENDING'
            AND size_bytes IS NULL
            AND content_type IS NULL
            AND confirmed_at IS NULL
        )
        OR
        (
            status = 'READY'
            AND size_bytes IS NOT NULL
            AND size_bytes BETWEEN 1 AND 8388608
            AND content_type IS NOT NULL
            AND confirmed_at IS NOT NULL
        )
    )
);

COMMIT;