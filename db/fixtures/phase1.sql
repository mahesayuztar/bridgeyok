BEGIN;

CREATE TEMPORARY TABLE phase1_fixture (
    id bigint PRIMARY KEY,
    label text NOT NULL
) ON COMMIT DROP;

INSERT INTO phase1_fixture (id, label)
VALUES (1, 'database-ready');

DO $$
BEGIN
    IF (SELECT count(*) FROM phase1_fixture) <> 1 THEN
        RAISE EXCEPTION 'phase1 fixture validation failed';
    END IF;
END
$$;

ROLLBACK;
