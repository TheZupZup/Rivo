-- Privilege tests for the API's database role.
--
-- The triggers on audit_events stop accidents, but the table owner can drop a
-- trigger. What makes the audit log genuinely append-only is that the role the API
-- connects as was never granted UPDATE, DELETE or TRUNCATE on it.
--
-- Run as rivo_app, against a database where database/roles/app_role.sql has been
-- applied:
--
--   psql "postgres://rivo_app:...@host/rivo" -f database/tests/app_role.sql
--
-- Everything runs inside a transaction that is rolled back.

\set ON_ERROR_STOP on

BEGIN;

\ir assertions.sql

DO $$
BEGIN
    IF current_user <> 'rivo_app' THEN
        RAISE EXCEPTION 'FAIL these assertions must run as rivo_app, not %', current_user;
    END IF;

    RAISE NOTICE 'ok  connected as %', current_user;
END $$;

-- ---------------------------------------------------------------------------
-- What the API must be able to do.
-- ---------------------------------------------------------------------------

SELECT pg_temp.assert_accepted(
    $$SELECT 1 FROM api_tokens LIMIT 1$$,
    'the api can read tokens to authenticate a request');

SELECT pg_temp.assert_accepted($$
    INSERT INTO reports (reporter_user_id, video_id, video_revision_id, reported_rule_code)
    VALUES ('44444444-4444-4444-8444-444444444444',
            '66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            'R-4')
$$, 'the api can file a report');

SELECT pg_temp.assert_accepted($$
    INSERT INTO audit_events (actor_user_id, event_type, subject_type, subject_id)
    VALUES ('44444444-4444-4444-8444-444444444444',
            'report.queued_for_review',
            'report',
            '66666666-6666-4666-8666-666666666666')
$$, 'the api can append an audit event');

-- ---------------------------------------------------------------------------
-- What no amount of application code should be able to do.
--
-- 42501 is insufficient_privilege. A different code here would mean the statement
-- was stopped by something droppable, such as a trigger, rather than by a grant
-- the role never held.
-- ---------------------------------------------------------------------------

SELECT pg_temp.assert_rejected(
    $$UPDATE audit_events SET event_type = 'tampered'$$,
    '42501', 'the api cannot rewrite an audit event');

SELECT pg_temp.assert_rejected(
    $$DELETE FROM audit_events$$,
    '42501', 'the api cannot delete an audit event');

SELECT pg_temp.assert_rejected(
    $$TRUNCATE audit_events$$,
    '42501', 'the api cannot truncate the audit log');

SELECT pg_temp.assert_rejected(
    $$UPDATE rulesets SET version = 'v9.9'$$,
    '42501', 'the api cannot edit a published ruleset');

SELECT pg_temp.assert_rejected($$
    INSERT INTO rules (ruleset_id, code, title, description)
    VALUES ('11111111-1111-4111-8111-111111111111', 'R-99', 'Backdated rule', 'Added after the fact.')
$$, '42501', 'the api cannot backdate a rule into an old ruleset');

SELECT pg_temp.assert_rejected($$
    INSERT INTO api_tokens (user_id, name, token_hash)
    VALUES ('33333333-3333-4333-8333-333333333333', 'forged', sha256('forged'::bytea))
$$, '42501', 'the api cannot mint itself a token');

ROLLBACK;
