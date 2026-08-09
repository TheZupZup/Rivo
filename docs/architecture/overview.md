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
   +---- PostgreSQL
   |
   +---- VideoStorage interface
             |
             +---- Local filesystem (v0.1)
             +---- S3-compatible storage (future)
```

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
