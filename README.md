# Rivo

An open-source video platform built around creator trust, predictable moderation,
clear appeals, and non-retroactive policy enforcement.

## Core principles

- A report is not a conviction.
- Policy changes do not retroactively punish content that complied with the rules in effect when it was published.
- Major moderation actions must be explainable and auditable.
- Permanent bans require more scrutiny than ordinary moderation actions.
- Legal removals and platform-policy violations are recorded as different events.
- The storage layer must remain replaceable so local storage can later move to S3-compatible infrastructure without rewriting the product.

## Repository layout

```text
apps/web/                  Next.js + TypeScript web client
services/api/              Go HTTP API
database/migrations/       PostgreSQL schema migrations
database/seed/             Development seed data
database/roles/            Least-privilege database role for the API
database/tests/            Assertions that the moderation invariants hold
docs/architecture/         Architecture decisions and system design
docs/moderation/           Moderation policy and invariants
deploy/                    Local deployment configuration
scripts/                   End-to-end smoke test
.github/workflows/         Continuous integration
```

## Current milestone: v0.1 foundation

The first milestone intentionally does not include live streaming, ads, 4K, or a recommendation algorithm.

It focuses on:

| | Area | State |
| --- | --- | --- |
| 1 | accounts and channels | schema only; identity comes from out-of-band API tokens |
| 2 | video metadata | schema only; uploads do not yet create a video record |
| 3 | versioned community rules | schema, seeded for development |
| 4 | non-retroactive report validation | working end to end at `POST /api/reports` |
| 5 | moderation cases and appeals | schema and constraints only |
| 6 | immutable audit records | working: report decisions are audited, `audit_events` is append-only |
| 7 | a replaceable video-storage boundary | working: `storage.VideoStore` with a local implementation |

[`docs/architecture/feasibility.md`](docs/architecture/feasibility.md) is an honest
assessment of what this project can and cannot become, and of what the sequencing
should be from here.

## Local development

### Requirements

- Go 1.23+
- Node.js 22+
- Docker + Docker Compose

### Start PostgreSQL

```bash
docker compose -f deploy/compose.yaml up -d postgres
```

Compose applies `database/migrations/` the first time the volume is created. Load
the development seed too, so there is a ruleset, a video and an API token to work
with:

```bash
make db-seed
```

### Run the API

```bash
cd services/api
cp .env.example .env
go run ./cmd/server
```

The API listens on `http://localhost:8080` by default. It refuses to start if it
cannot reach the database, so a misconfigured `DATABASE_URL` fails immediately
rather than on the first user request.

Health check:

```bash
curl http://localhost:8080/healthz
```

### Run the web app

```bash
cd apps/web
npm install
npm run dev
```

The web app listens on `http://localhost:3000`.

### Authentication

Every write route requires a bearer token. v0.1 has no signup flow, so tokens are
issued out of band and only their SHA-256 digest is stored. The development seed
creates two:

| Token | User |
| --- | --- |
| `rivo_dev_creator_token` | `demo-creator` |
| `rivo_dev_reporter_token` | `demo-reporter` |

These are published here on purpose. They are local throwaway credentials, and they
must never exist in a deployment that matters.

### Test a local video upload

The upload prototype streams a multipart video directly into the configured local
`VideoStore`. The container is identified from the file's own bytes, so renaming
something to `.mp4` will not get it accepted.

```bash
curl -X POST \
  -H "Authorization: Bearer rivo_dev_creator_token" \
  -F "video=@/path/to/video.mp4" \
  http://localhost:8080/api/videos
```

Or open `http://localhost:3000/upload`. If the API is running on a different port, configure the web app before starting it:

```bash
NEXT_PUBLIC_RIVO_API_URL=http://127.0.0.1:8081 npm run dev
```

Uploaded sources are stored under `VIDEO_STORAGE_PATH/videos/<video-id>/source`. FFmpeg transcoding and playback are intentionally the next milestone.

### Test non-retroactive enforcement

The seed publishes a video in 2026 under ruleset `v1.3`, and edits its title in 2027
under `v2.0`. Rule `R-17` only exists in `v2.0`.

Reporting the 2026 media under `R-17` is refused before any moderation case can
exist:

```bash
curl -X POST \
  -H "Authorization: Bearer rivo_dev_reporter_token" \
  -H "Content-Type: application/json" \
  -d '{"videoId":"66666666-6666-4666-8666-666666666666","ruleCode":"R-17"}' \
  http://localhost:8080/api/reports
```

```json
{
  "id": "...",
  "status": "rejected_not_applicable",
  "rulesetVersion": "v1.3",
  "revisionKind": "media",
  "reason": "reported rule was not applicable to this content revision"
}
```

The same rule against the title edited in 2027 is accepted, because that revision
really was created under `v2.0`:

```bash
curl -X POST \
  -H "Authorization: Bearer rivo_dev_reporter_token" \
  -H "Content-Type: application/json" \
  -d '{"videoId":"66666666-6666-4666-8666-666666666666","revisionKind":"title","ruleCode":"R-17"}' \
  http://localhost:8080/api/reports
```

Both outcomes are recorded, and both write an `audit_events` row in the same
transaction as the report. A rejected report is still a report: refusing to record
it would make the rejection itself unreviewable.

## Moderation invariant

A rule may only be used for a creator-policy sanction when that rule was active for the revision being judged.

For example, if a video was published under ruleset `v1.3`, a rule introduced in `v2.0` cannot produce a strike against that original upload.

The database enforces this rather than trusting application code: a moderation case
names the exact revision it judges, and a composite foreign key forces its
`applicable_ruleset_id` to be that revision's ruleset. A case that applies a newer
rule to an older revision cannot be written at all.

See [`docs/moderation/non-retroactive-enforcement.md`](docs/moderation/non-retroactive-enforcement.md).

## Checks

```bash
make lint-ci     # the workflow files parse and the shell scripts lint
make api-check   # gofmt, vet, tests under the race detector
make db-verify   # the moderation invariants are enforced by the schema
make smoke       # the real binary, driven over HTTP against a real database
make check       # all of the above
```

The same checks run in CI on every pull request. See
[`CONTRIBUTING.md`](CONTRIBUTING.md#continuous-integration).

## License

Rivo is intended to be licensed under **GNU AGPL v3 only (`AGPL-3.0-only`)** so improvements to the network service remain available to the community.

## Contributing

Rivo is being built in public. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the initial contribution workflow.
