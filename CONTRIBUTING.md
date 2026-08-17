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
- `database/seed/` — development seed data
- `docs/moderation/` — moderation invariants and policy design
- `docs/architecture/` — technical architecture
- `deploy/` — local development infrastructure

## Quality expectations

- Prefer simple code over clever abstractions.
- Add tests for domain rules and bug fixes.
- Never trust moderation-critical facts supplied by the browser when the server can derive them.
- Express moderation invariants as database constraints where a constraint can express them. Application checks are for good error messages; the schema is what makes a bad state impossible.
- Keep storage and infrastructure replaceable behind narrow interfaces.
- Explain behavior changes in the pull request description.

## Running tests

```bash
make api-test
```

Before opening a pull request, run the full check — formatting, vet and tests:

```bash
make api-check
```

Changes touching the schema should be verified against a real database, not only
through unit tests:

```bash
make db-up
make db-reset
make db-verify
```

`make db-verify` asserts that the moderation invariants really are enforced by the
schema. If you add a constraint that encodes a policy, add an assertion for it in
`database/tests/constraints.sql` — a constraint nobody tests is a constraint a
future migration can quietly drop.

To run everything CI runs, against a database started with `make db-up`:

```bash
make check
```

## Continuous integration

`.github/workflows/ci.yml` runs three jobs on every pull request:

| Job | What it proves |
| --- | --- |
| Workflows and scripts | the workflow files parse and the shell scripts lint |
| Go API | formatted, vets clean, tests pass under the race detector, builds, dependencies tidy |
| Database | migrations apply in order, the seed is idempotent, every moderation invariant is enforced, the application role cannot rewrite history, and the real binary behaves correctly over HTTP |
| Web app | installs from the lockfile, typechecks, builds |

Branch protection should require the single `CI` check, which passes only when
every job succeeds. Adding a job above does not mean editing repository settings.

Run `make lint-ci` before pushing a change to `.github/workflows/`. A workflow file
that does not parse never runs, so CI cannot be the thing that tells you it is
broken — the run fails with no jobs and no explanation.

Web dependencies install with `npm ci` from `package-lock.json`. If you change
`apps/web/package.json`, commit the updated lockfile with it.

## Commit messages

Use concise conventional-style messages when practical, for example:

```text
feat: add video upload session
fix: reject rules that did not apply at publication
chore: update local development config
```
