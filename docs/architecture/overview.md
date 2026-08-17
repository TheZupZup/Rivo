# Architecture overview

The first version is deliberately small.

```text
Browser
   |
   v
Next.js web app
   |
   v
Go HTTP API
   |
   +---- rate limit (per client address)
   |        |
   |        v
   |     authentication (bearer token -> user)
   |        |
   |        v
   |     handlers -> domain services
   |
   +---- PostgreSQL
   |
   +---- VideoStorage interface
             |
             +---- Local filesystem (v0.1)
             +---- S3-compatible storage (future)
```

Rate limiting runs before authentication on purpose: an unauthenticated flood must
not be able to force one database lookup per request.

## Boundaries

### Web client

Responsible for presentation and user interaction. It does not contain moderation rules or storage policy.

### API

Owns business rules, including moderation invariants and report validation.

### PostgreSQL

Stores durable product data and moderation history.

### Video storage

Accessed through a narrow interface so the application can start with local disks and later move to object storage without changing domain logic.

## Trust boundary

The browser never tells the API which rules were applicable to a video.

The API resolves the video's content revision and ruleset from server-owned data before deciding whether a report is eligible for review. This prevents a modified client from weakening moderation invariants.

Concretely, a report request carries only a video id, an optional revision kind and a
rule code. Everything else is derived:

| Fact | Source |
| --- | --- |
| who is reporting | the bearer token, never the request body |
| which revision is judged | resolved from the database for that video and kind |
| which ruleset applies | frozen on that revision when it was created |
| what a stored file is | the uploaded bytes, never the declared `Content-Type` |

The request decoder rejects unknown fields, so a client that tries to send a
`rulesetId` or a `reporterUserId` is refused rather than silently ignored.

## Defence in depth

Application code is not the last line of defence for moderation facts. The
constraints in `database/migrations/002_moderation_integrity.sql` make the central
invariants unrepresentable rather than merely unwritten: a moderation case cannot
name a ruleset other than the one frozen on the revision it judges, a report cannot
cite a revision belonging to a different video, two rulesets cannot be in effect at
once, and `audit_events` rejects `UPDATE`, `DELETE` and `TRUNCATE`.

`database/roles/app_role.sql` completes that last one. A trigger stops accidents;
only a connection role that was never granted the privilege stops a future service
from deciding to tidy up history.
