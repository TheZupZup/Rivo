-- Moderation integrity constraints.
--
-- 001 modelled the moderation domain but left the invariants to application code.
-- This migration moves the ones that are cheap to express into the database, so a
-- bug, a manual psql session, or a future service cannot produce moderation history
-- that contradicts the published policy.
--
-- Safe to apply on a v0.1 database: no service writes to moderation_cases yet, so
-- the NOT NULL column added below cannot conflict with existing rows.

BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ---------------------------------------------------------------------------
-- Only one ruleset may be in effect at any instant.
--
-- Without this, two overlapping rulesets make "the rules in effect when this was
-- published" ambiguous, and the whole non-retroactivity promise becomes unprovable.
-- A NULL effective_until means "still in effect", so two open-ended rulesets
-- overlap and are rejected.
-- ---------------------------------------------------------------------------

ALTER TABLE rulesets
    ADD CONSTRAINT rulesets_effective_period_no_overlap
    EXCLUDE USING gist (tstzrange(effective_from, effective_until) WITH &&);

-- ---------------------------------------------------------------------------
-- A report must name a revision that genuinely belongs to the video it names.
--
-- 001 stored video_id and video_revision_id independently, so a report could point
-- at video A while citing a revision of video B — and the ruleset resolved for
-- eligibility would then be the wrong one.
-- ---------------------------------------------------------------------------

ALTER TABLE video_revisions
    ADD CONSTRAINT video_revisions_id_video_id_key UNIQUE (id, video_id);

ALTER TABLE reports
    DROP CONSTRAINT reports_video_revision_id_fkey;

ALTER TABLE reports
    ADD CONSTRAINT reports_revision_belongs_to_video_fkey
    FOREIGN KEY (video_revision_id, video_id)
    REFERENCES video_revisions (id, video_id)
    ON DELETE CASCADE;

-- A rejected report must say why it was rejected.
ALTER TABLE reports
    ADD CONSTRAINT reports_rejection_reason_required
    CHECK (status <> 'rejected_not_applicable' OR rejection_reason IS NOT NULL);

CREATE INDEX reports_status_idx ON reports (status);
CREATE INDEX reports_video_revision_id_idx ON reports (video_revision_id);

-- ---------------------------------------------------------------------------
-- A moderation case judges one specific revision, under that revision's ruleset.
--
-- This is the database-level form of the core product promise. applicable_ruleset_id
-- is no longer a free field a moderator or a future service can set to a newer
-- ruleset: the composite foreign key below forces it to equal the ruleset frozen on
-- the revision being judged.
--
-- It also makes enforcement scope explicit. A case opened against a 2027 title edit
-- names the title revision, so it can never be presented as a judgement of media
-- published in 2026 under an older ruleset.
-- ---------------------------------------------------------------------------

ALTER TABLE video_revisions
    ADD CONSTRAINT video_revisions_id_ruleset_id_key UNIQUE (id, ruleset_id);

ALTER TABLE moderation_cases
    ADD COLUMN video_revision_id UUID NOT NULL;

ALTER TABLE moderation_cases
    ADD CONSTRAINT moderation_cases_revision_belongs_to_video_fkey
    FOREIGN KEY (video_revision_id, video_id)
    REFERENCES video_revisions (id, video_id)
    ON DELETE CASCADE;

ALTER TABLE moderation_cases
    ADD CONSTRAINT moderation_cases_ruleset_matches_revision_fkey
    FOREIGN KEY (video_revision_id, applicable_ruleset_id)
    REFERENCES video_revisions (id, ruleset_id);

-- A decision is not a decision until it is dated, and a dated case must have decided.
ALTER TABLE moderation_cases
    ADD CONSTRAINT moderation_cases_decision_dated
    CHECK ((decision IS NULL) = (decided_at IS NULL));

CREATE INDEX moderation_cases_video_revision_id_idx ON moderation_cases (video_revision_id);

-- ---------------------------------------------------------------------------
-- audit_events: append-only for real.
--
-- 001 used a row-level trigger, which TRUNCATE bypasses entirely — row triggers do
-- not fire for TRUNCATE, so the "immutable" audit log could be emptied in one
-- statement. A statement-level trigger closes that hole.
--
-- Privileges are the other half: see database/roles/app_role.sql for the
-- least-privilege role the API should connect as. A trigger stops accidents; only
-- revoked privileges stop an application bug that decides to "clean up" history.
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION prevent_audit_event_truncate()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_truncate
BEFORE TRUNCATE ON audit_events
FOR EACH STATEMENT EXECUTE FUNCTION prevent_audit_event_truncate();

REVOKE UPDATE, DELETE, TRUNCATE ON audit_events FROM PUBLIC;

CREATE INDEX audit_events_created_at_idx ON audit_events (created_at DESC);

-- ---------------------------------------------------------------------------
-- API tokens.
--
-- v0.1 has no signup flow, but every moderation-critical write needs an actor: a
-- report has to record who filed it, and an upload has to record who published it.
-- Tokens are issued out of band (see the dev seed) and only their SHA-256 digest is
-- stored, so a database leak does not hand out working credentials.
-- ---------------------------------------------------------------------------

CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    CHECK (length(token_hash) = 32)
);

CREATE INDEX api_tokens_user_id_idx ON api_tokens (user_id);

COMMIT;
