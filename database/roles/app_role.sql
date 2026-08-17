-- Least-privilege database role for the Rivo API.
--
-- Why this exists
--
-- audit_events carries a trigger that raises on UPDATE, DELETE and TRUNCATE, but a
-- trigger only protects against accidents: the table owner can drop it. For the
-- audit log to be genuinely append-only, the API must connect as a role that is not
-- the owner and that has never been granted the privileges in question.
--
-- This file is not a migration. It is applied once by an operator, because it needs
-- a password that must not live in the repository.
--
-- Usage:
--
--   psql "$ADMIN_DATABASE_URL" \
--     -v app_password="$(openssl rand -hex 32)" \
--     -f database/roles/app_role.sql
--
-- Then point the API at the new role:
--
--   DATABASE_URL=postgres://rivo_app:<password>@localhost:5432/rivo?sslmode=disable

BEGIN;

SELECT format('CREATE ROLE rivo_app LOGIN PASSWORD %L', :'app_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'rivo_app')
\gexec

GRANT CONNECT ON DATABASE rivo TO rivo_app;
GRANT USAGE ON SCHEMA public TO rivo_app;

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO rivo_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO rivo_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO rivo_app;

-- The audit log is the exception: insert and read only, no rewriting history.
REVOKE UPDATE, DELETE, TRUNCATE ON audit_events FROM rivo_app;

-- Rulesets and rules are edited through an administrative path, never by the API.
REVOKE INSERT, UPDATE, DELETE ON rulesets, rules FROM rivo_app;

-- Tokens are issued out of band; the API only ever reads them.
REVOKE INSERT, UPDATE, DELETE ON api_tokens FROM rivo_app;

COMMIT;
