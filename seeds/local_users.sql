BEGIN;

INSERT INTO users (id, email)
VALUES
    (
        '11111111-1111-4111-8111-111111111111',
        'alice@cloudtasks.invalid'
    ),
    (
        '22222222-2222-4222-8222-222222222222',
        'bob@cloudtasks.invalid'
    )
ON CONFLICT (id) DO NOTHING;

COMMIT;