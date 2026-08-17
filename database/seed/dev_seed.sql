-- Local development seed.
--
-- Creates enough of a world to exercise the non-retroactivity invariant end to end:
-- two rulesets, a video published under the older one, and a title edited under the
-- newer one.
--
-- DO NOT LOAD THIS ANYWHERE REAL. The API tokens below are published in this
-- repository and are therefore public credentials.
--
--   make db-seed
--
-- Re-running is safe: every insert is idempotent on its natural key.

BEGIN;

-- ---------------------------------------------------------------------------
-- Rulesets. v1.3 ran until v2.0 replaced it; the EXCLUDE constraint added in
-- migration 002 means these periods may not overlap.
-- ---------------------------------------------------------------------------

INSERT INTO rulesets (id, version, effective_from, effective_until) VALUES
    ('11111111-1111-4111-8111-111111111111', 'v1.3', '2026-01-01T00:00:00Z', '2027-02-01T00:00:00Z'),
    ('22222222-2222-4222-8222-222222222222', 'v2.0', '2027-02-01T00:00:00Z', NULL)
ON CONFLICT (version) DO NOTHING;

INSERT INTO rules (ruleset_id, code, title, description) VALUES
    ('11111111-1111-4111-8111-111111111111', 'R-4', 'Harassment',
     'Targeted abuse directed at an identifiable person.'),
    ('11111111-1111-4111-8111-111111111111', 'R-9', 'Impersonation',
     'Passing yourself off as another person or organisation.'),
    -- v2.0 carries the older rules forward and introduces R-17.
    ('22222222-2222-4222-8222-222222222222', 'R-4', 'Harassment',
     'Targeted abuse directed at an identifiable person.'),
    ('22222222-2222-4222-8222-222222222222', 'R-9', 'Impersonation',
     'Passing yourself off as another person or organisation.'),
    ('22222222-2222-4222-8222-222222222222', 'R-17', 'Synthetic media disclosure',
     'Generated or materially altered media must be disclosed. Introduced in v2.0.')
ON CONFLICT (ruleset_id, code) DO NOTHING;

-- ---------------------------------------------------------------------------
-- People.
-- ---------------------------------------------------------------------------

INSERT INTO users (id, handle) VALUES
    ('33333333-3333-4333-8333-333333333333', 'demo-creator'),
    ('44444444-4444-4444-8444-444444444444', 'demo-reporter')
ON CONFLICT (handle) DO NOTHING;

INSERT INTO channels (id, owner_user_id, slug, display_name) VALUES
    ('55555555-5555-4555-8555-555555555555', '33333333-3333-4333-8333-333333333333',
     'demo-channel', 'Demo Channel')
ON CONFLICT (slug) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Content.
--
-- The media was published in 2026 under v1.3. The title was corrected in 2027, so
-- that revision is bound to v2.0. This is the exact situation the invariant exists
-- for: R-17 may be applied to the 2027 title, never to the 2026 media.
-- ---------------------------------------------------------------------------

INSERT INTO videos (id, channel_id, title, description, published_at, created_at) VALUES
    ('66666666-6666-4666-8666-666666666666', '55555555-5555-4555-8555-555555555555',
     'A video published under ruleset v1.3', 'Seeded for local development.',
     '2026-09-10T12:00:00Z', '2026-09-10T12:00:00Z')
ON CONFLICT (id) DO NOTHING;

INSERT INTO video_revisions (id, video_id, revision_number, revision_kind, ruleset_id, created_at) VALUES
    ('77777777-7777-4777-8777-777777777777', '66666666-6666-4666-8666-666666666666',
     1, 'media', '11111111-1111-4111-8111-111111111111', '2026-09-10T12:00:00Z'),
    ('88888888-8888-4888-8888-888888888888', '66666666-6666-4666-8666-666666666666',
     1, 'title', '22222222-2222-4222-8222-222222222222', '2027-03-04T09:30:00Z')
ON CONFLICT (video_id, revision_kind, revision_number) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Development API tokens.
--
-- Only the SHA-256 digest is stored. The plaintext values are:
--
--   creator  : rivo_dev_creator_token
--   reporter : rivo_dev_reporter_token
--
-- Publishing the plaintext here is the point — these are throwaway local
-- credentials, and a seed nobody can use is a seed nobody runs.
-- ---------------------------------------------------------------------------

INSERT INTO api_tokens (user_id, name, token_hash) VALUES
    ('33333333-3333-4333-8333-333333333333', 'local development creator',
     '\xfb1bb7882af942b2ed8315c54b834cd5dfbb6c94efcf89199f37ad739048c46b'::bytea),
    ('44444444-4444-4444-8444-444444444444', 'local development reporter',
     '\xde78ae158451d255ca60abc26edc717d7461d233fb7282e542a893c563c916f0'::bytea)
ON CONFLICT (token_hash) DO NOTHING;

COMMIT;
