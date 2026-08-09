# Contributing to Rivo

Rivo is early-stage software. The goal is to keep contributions small, reviewable, and easy to understand.

## Before coding

1. Check existing issues and pull requests to avoid duplicated work.
2. For large changes, open an issue first and describe the problem and proposed approach.
3. Keep pull requests focused on one responsibility.

## Development areas

- `apps/web/` — Next.js and TypeScript user interface
- `services/api/` — Go API and domain logic
- `database/migrations/` — PostgreSQL schema
- `docs/moderation/` — moderation invariants and policy design
- `docs/architecture/` — technical architecture
- `deploy/` — local development infrastructure

## Quality expectations

- Prefer simple code over clever abstractions.
- Add tests for domain rules and bug fixes.
- Never trust moderation-critical facts supplied by the browser when the server can derive them.
- Keep storage and infrastructure replaceable behind narrow interfaces.
- Explain behavior changes in the pull request description.

## Running tests

```bash
make api-test
```

## Commit messages

Use concise conventional-style messages when practical, for example:

```text
feat: add video upload session
fix: reject rules that did not apply at publication
chore: update local development config
```
