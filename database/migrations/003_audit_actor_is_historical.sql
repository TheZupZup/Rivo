-- Make the audit log survive account deletion.
--
-- 001 gave audit_events.actor_user_id an ON DELETE SET NULL foreign key to users,
-- and also made audit_events reject UPDATE. Those two are contradictory: the
-- cascade has to issue an UPDATE on audit_events to null the column, the trigger
-- refuses it, and the whole DELETE fails.
--
-- The consequence is not a corner case. No user who has ever acted — filed a
-- report, uploaded a video, taken a moderation decision — could be deleted at all.
-- Account deletion was impossible, and so was satisfying an erasure request.
--
-- The fix is to stop treating the actor as a live foreign key. An audit log records
-- what happened; it is not a view of who currently exists. Keeping the reference as
-- a plain identifier means deleting a user leaves their past actions recorded
-- against an id that no longer resolves, which is the correct outcome: the event
-- genuinely occurred, and the log must not be rewritten to pretend otherwise.
--
-- This is the ordinary shape for an append-only log. A log that carries foreign
-- keys into mutable tables is a log that later changes can reshape.

BEGIN;

ALTER TABLE audit_events
    DROP CONSTRAINT audit_events_actor_user_id_fkey;

COMMENT ON COLUMN audit_events.actor_user_id IS
    'The user who performed the action, recorded as a historical fact. Deliberately '
    'not a foreign key: the audit log must outlive the accounts it describes, and '
    'nulling this column on account deletion would be a write to an append-only '
    'table. An id here may no longer resolve to a row in users.';

CREATE INDEX audit_events_actor_user_id_idx ON audit_events (actor_user_id);

COMMIT;
