-- Regression tests for the moderation invariants.
--
-- The constraints in 002 are the product promise expressed as schema: a rule that
-- did not exist cannot be applied, a report cannot cite another video's revision,
-- history cannot be rewritten. A future migration that drops one of them would be a
-- silent policy change, so each is asserted here rather than trusted.
--
-- Run against a migrated and seeded database:
--
--   make db-verify
--
-- Every statement runs inside a transaction that is rolled back, so this leaves the
-- database exactly as it found it.

\set ON_ERROR_STOP on

BEGIN;

\ir assertions.sql

-- ---------------------------------------------------------------------------
-- The seed is the fixture. If it stops describing a 2026 upload judged under
-- v1.3 and a 2027 title edit judged under v2.0, every assertion below becomes
-- meaningless, so check it first.
-- ---------------------------------------------------------------------------

DO $$
DECLARE
    media_ruleset text;
    title_ruleset text;
BEGIN
    SELECT rulesets.version INTO media_ruleset
    FROM video_revisions
    JOIN rulesets ON rulesets.id = video_revisions.ruleset_id
    WHERE video_revisions.id = '77777777-7777-4777-8777-777777777777';

    SELECT rulesets.version INTO title_ruleset
    FROM video_revisions
    JOIN rulesets ON rulesets.id = video_revisions.ruleset_id
    WHERE video_revisions.id = '88888888-8888-4888-8888-888888888888';

    IF media_ruleset IS DISTINCT FROM 'v1.3' OR title_ruleset IS DISTINCT FROM 'v2.0' THEN
        RAISE EXCEPTION 'FAIL seed fixture (media ruleset %, title ruleset %)', media_ruleset, title_ruleset;
    END IF;

    IF EXISTS (
        SELECT 1 FROM rules
        WHERE ruleset_id = '11111111-1111-4111-8111-111111111111' AND code = 'R-17'
    ) THEN
        RAISE EXCEPTION 'FAIL seed fixture (R-17 must not exist in v1.3)';
    END IF;

    RAISE NOTICE 'ok  seed describes a 2026 upload under v1.3 and a 2027 title under v2.0';
END $$;

-- ---------------------------------------------------------------------------
-- Exactly one ruleset may be in effect at any instant.
-- ---------------------------------------------------------------------------

SELECT pg_temp.assert_rejected($$
    INSERT INTO rulesets (version, effective_from, effective_until)
    VALUES ('v1.5', '2026-06-01T00:00:00Z', '2027-06-01T00:00:00Z')
$$, '23P01', 'a ruleset overlapping an existing period is rejected');

SELECT pg_temp.assert_rejected($$
    INSERT INTO rulesets (version, effective_from, effective_until)
    VALUES ('v3.0', '2028-01-01T00:00:00Z', NULL)
$$, '23P01', 'a second open-ended ruleset is rejected');

-- ---------------------------------------------------------------------------
-- A report must name a revision that belongs to the video it names, or the
-- ruleset resolved for eligibility would be the wrong one.
-- ---------------------------------------------------------------------------

INSERT INTO videos (id, channel_id, title)
VALUES ('99999999-9999-4999-8999-999999999999',
        '55555555-5555-4555-8555-555555555555',
        'A different video');

SELECT pg_temp.assert_rejected($$
    INSERT INTO reports (reporter_user_id, video_id, video_revision_id, reported_rule_code)
    VALUES ('44444444-4444-4444-8444-444444444444',
            '99999999-9999-4999-8999-999999999999',
            '77777777-7777-4777-8777-777777777777',
            'R-4')
$$, '23503', 'a report citing another video''s revision is rejected');

SELECT pg_temp.assert_accepted($$
    INSERT INTO reports (reporter_user_id, video_id, video_revision_id, reported_rule_code)
    VALUES ('44444444-4444-4444-8444-444444444444',
            '66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            'R-4')
$$, 'a report citing its own video''s revision is accepted');

SELECT pg_temp.assert_rejected($$
    INSERT INTO reports (reporter_user_id, video_id, video_revision_id, reported_rule_code)
    VALUES ('44444444-4444-4444-8444-444444444444',
            '66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            'R-4')
$$, '23505', 'the same reporter cannot file the same allegation twice');

SELECT pg_temp.assert_rejected($$
    INSERT INTO reports (reporter_user_id, video_id, video_revision_id, reported_rule_code, status)
    VALUES ('44444444-4444-4444-8444-444444444444',
            '66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            'R-9',
            'rejected_not_applicable')
$$, '23514', 'a rejected report must carry a reason');

-- ---------------------------------------------------------------------------
-- The core promise: a case is judged under the ruleset frozen on the revision it
-- names, and nothing else.
-- ---------------------------------------------------------------------------

SELECT pg_temp.assert_rejected($$
    INSERT INTO moderation_cases (video_id, video_revision_id, applicable_ruleset_id, rule_code)
    VALUES ('66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            '22222222-2222-4222-8222-222222222222',
            'R-17')
$$, '23503', 'a 2027 rule cannot be applied to media published under v1.3');

SELECT pg_temp.assert_accepted($$
    INSERT INTO moderation_cases (video_id, video_revision_id, applicable_ruleset_id, rule_code)
    VALUES ('66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            '11111111-1111-4111-8111-111111111111',
            'R-4')
$$, 'a case judging media under its own ruleset is accepted');

-- Metadata edited in 2027 genuinely was created under v2.0, so R-17 applies to
-- that revision — and only to that revision.
SELECT pg_temp.assert_accepted($$
    INSERT INTO moderation_cases (video_id, video_revision_id, applicable_ruleset_id, rule_code)
    VALUES ('66666666-6666-4666-8666-666666666666',
            '88888888-8888-4888-8888-888888888888',
            '22222222-2222-4222-8222-222222222222',
            'R-17')
$$, 'a 2027 rule may be applied to a title edited in 2027');

SELECT pg_temp.assert_rejected($$
    INSERT INTO moderation_cases (video_id, video_revision_id, applicable_ruleset_id, rule_code)
    VALUES ('99999999-9999-4999-8999-999999999999',
            '77777777-7777-4777-8777-777777777777',
            '11111111-1111-4111-8111-111111111111',
            'R-4')
$$, '23503', 'a case citing another video''s revision is rejected');

SELECT pg_temp.assert_rejected($$
    INSERT INTO moderation_cases (video_id, video_revision_id, applicable_ruleset_id, rule_code, decision)
    VALUES ('66666666-6666-4666-8666-666666666666',
            '77777777-7777-4777-8777-777777777777',
            '11111111-1111-4111-8111-111111111111',
            'R-9',
            'warning')
$$, '23514', 'a decision without a decision date is rejected');

-- ---------------------------------------------------------------------------
-- The audit log is append-only.
-- ---------------------------------------------------------------------------

INSERT INTO audit_events (event_type, subject_type, subject_id)
VALUES ('test.event', 'video', '66666666-6666-4666-8666-666666666666');

SELECT pg_temp.assert_rejected(
    $$UPDATE audit_events SET event_type = 'tampered'$$,
    'P0001', 'audit events cannot be updated');

SELECT pg_temp.assert_rejected(
    $$DELETE FROM audit_events$$,
    'P0001', 'audit events cannot be deleted');

-- The row-level trigger in 001 did not cover this: TRUNCATE does not fire row
-- triggers, so the whole log could be dropped in one statement.
SELECT pg_temp.assert_rejected(
    $$TRUNCATE audit_events$$,
    'P0001', 'audit events cannot be truncated');

-- ---------------------------------------------------------------------------
-- Append-only must not mean "accounts can never be deleted".
--
-- These two requirements pulled against each other until 003: nulling the actor
-- on account deletion is a write to an append-only table, so the delete failed and
-- no user who had ever acted could be removed. The audit entry has to survive the
-- account without blocking its deletion.
-- ---------------------------------------------------------------------------

DO $$
DECLARE
    actor_id uuid;
    surviving_events integer;
BEGIN
    INSERT INTO users (handle) VALUES ('constraint-test-actor') RETURNING id INTO actor_id;

    INSERT INTO audit_events (actor_user_id, event_type, subject_type, subject_id)
    VALUES (actor_id, 'test.event', 'video', '66666666-6666-4666-8666-666666666666');

    DELETE FROM users WHERE id = actor_id;
    RAISE NOTICE 'ok  a user who has acted can still be deleted';

    SELECT count(*) INTO surviving_events FROM audit_events WHERE actor_user_id = actor_id;
    IF surviving_events <> 1 THEN
        RAISE EXCEPTION 'FAIL the audit event must outlive the account (found % events)', surviving_events;
    END IF;

    RAISE NOTICE 'ok  their audit events outlive the account';
END $$;

-- ---------------------------------------------------------------------------
-- Tokens are stored as digests, never as anything a leak could replay.
-- ---------------------------------------------------------------------------

SELECT pg_temp.assert_rejected($$
    INSERT INTO api_tokens (user_id, name, token_hash)
    VALUES ('33333333-3333-4333-8333-333333333333', 'plaintext', 'not-a-digest'::bytea)
$$, '23514', 'an api token that is not a 32 byte digest is rejected');

ROLLBACK;
