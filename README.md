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
docs/architecture/         Architecture decisions and system design
docs/moderation/           Moderation policy and invariants
deploy/                    Local deployment configuration
```

## Current milestone: v0.1 foundation

The first milestone intentionally does not include live streaming, ads, 4K, or a recommendation algorithm.

It focuses on:

1. accounts and channels
2. video metadata
3. versioned community rules
4. non-retroactive report validation
5. moderation cases and appeals
6. immutable audit records
7. a replaceable video-storage boundary

## Local development

### Requirements

- Go 1.23+
- Node.js 22+
- Docker + Docker Compose

### Start PostgreSQL

```bash
docker compose -f deploy/compose.yaml up -d postgres
```

### Run the API

```bash
cd services/api
cp .env.example .env
go run ./cmd/server
```

The API listens on `http://localhost:8080` by default.

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

### Test a local video upload

The upload prototype streams a multipart video directly into the configured local `VideoStore`.

```bash
curl -X POST \
  -F "video=@/path/to/video.mp4" \
  http://localhost:8080/api/videos
```

Or open `http://localhost:3000/upload`. If the API is running on a different port, configure the web app before starting it:

```bash
NEXT_PUBLIC_RIVO_API_URL=http://127.0.0.1:8081 npm run dev
```

Uploaded sources are stored under `VIDEO_STORAGE_PATH/videos/<video-id>/source`. FFmpeg transcoding and playback are intentionally the next milestone.

## Moderation invariant

A rule may only be used for a creator-policy sanction when that rule was active for the revision being judged.

For example, if a video was published under ruleset `v1.3`, a rule introduced in `v2.0` cannot produce a strike against that original upload.

See [`docs/moderation/non-retroactive-enforcement.md`](docs/moderation/non-retroactive-enforcement.md).

## License

Rivo is intended to be licensed under **GNU AGPL v3 only (`AGPL-3.0-only`)** so improvements to the network service remain available to the community.

## Contributing

Rivo is being built in public. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the initial contribution workflow.
