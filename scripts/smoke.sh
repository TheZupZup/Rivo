#!/usr/bin/env bash
#
# End-to-end smoke test for the Rivo API.
#
# Unit tests use fakes for the stores, so they cannot catch a broken query, a
# constraint the code violates, or a route that is no longer protected. This
# starts the real binary against a real database and drives it over HTTP.
#
# Usage:
#
#   DATABASE_URL="postgres://rivo:rivo_dev@localhost:5432/rivo?sslmode=disable" \
#     scripts/smoke.sh
#
# The database must already be migrated. The script inserts its own fixture under
# freshly generated identifiers, so it is repeatable and does not depend on the
# development seed. That fixture is removed on exit, except for audit events, which
# are append-only by design.
#
# DATABASE_URL must name a role that can write fixtures (the owner, not rivo_app).

set -euo pipefail

REPOSITORY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPOSITORY_ROOT
readonly API_PORT="${SMOKE_API_PORT:-18099}"
readonly API_URL="http://127.0.0.1:${API_PORT}"

: "${DATABASE_URL:?DATABASE_URL must be set}"

failures=0
api_pid=""
work_directory="$(mktemp -d)"

cleanup() {
    if [[ -n "${api_pid}" ]] && kill -0 "${api_pid}" 2>/dev/null; then
        kill -TERM "${api_pid}" 2>/dev/null || true
        wait "${api_pid}" 2>/dev/null || true
    fi

    if [[ -n "${video_id:-}" ]]; then
        psql "${DATABASE_URL}" -q -c "DELETE FROM videos WHERE id = '${video_id}'" >/dev/null 2>&1 || true
        psql "${DATABASE_URL}" -q -c "DELETE FROM users WHERE id = '${user_id}'" >/dev/null 2>&1 || true
    fi

    rm -rf "${work_directory}"
}
trap cleanup EXIT

pass() {
    printf 'ok   %s\n' "$1"
}

fail() {
    printf 'FAIL %s\n' "$1"
    failures=$((failures + 1))
}

# expect_response METHOD PATH EXPECTED_STATUS DESCRIPTION [curl args...]
#
# Asserts the status code, and leaves the body in ${response_body} so a caller can
# make further assertions about it.
expect_response() {
    local method="$1" path="$2" expected_status="$3" description="$4"
    shift 4

    local status
    status="$(curl -s -o "${work_directory}/body" -w '%{http_code}' \
        -X "${method}" "${API_URL}${path}" "$@")"
    response_body="$(cat "${work_directory}/body")"

    if [[ "${status}" != "${expected_status}" ]]; then
        fail "${description} (expected HTTP ${expected_status}, got ${status}: ${response_body})"
        return
    fi

    pass "${description}"
}

expect_body_contains() {
    local needle="$1" description="$2"

    if [[ "${response_body}" != *"${needle}"* ]]; then
        fail "${description} (${needle} missing from ${response_body})"
        return
    fi

    pass "${description}"
}

# ---------------------------------------------------------------------------
# Fixture: a video published under the older ruleset, with a title edited under
# the newer one. Identifiers are generated so repeated runs never collide.
# ---------------------------------------------------------------------------

RUN_ID="$(od -An -tx1 -N6 /dev/urandom | tr -d ' \n')"
readonly RUN_ID
readonly API_TOKEN="smoke_${RUN_ID}"
TOKEN_DIGEST="$(printf '%s' "${API_TOKEN}" | sha256sum | cut -d' ' -f1)"
readonly TOKEN_DIGEST

echo "Creating fixture..."
fixture="$(psql "${DATABASE_URL}" -qtA -v ON_ERROR_STOP=1 <<SQL
WITH
old_ruleset AS (SELECT id FROM rulesets ORDER BY effective_from LIMIT 1),
new_ruleset AS (SELECT id FROM rulesets ORDER BY effective_from DESC LIMIT 1),
new_user AS (
    INSERT INTO users (handle) VALUES ('smoke-${RUN_ID}') RETURNING id
),
new_channel AS (
    INSERT INTO channels (owner_user_id, slug, display_name)
    SELECT id, 'smoke-${RUN_ID}', 'Smoke Test Channel' FROM new_user
    RETURNING id
),
new_video AS (
    INSERT INTO videos (channel_id, title, published_at)
    SELECT id, 'Smoke test video', NOW() FROM new_channel
    RETURNING id
),
media_revision AS (
    INSERT INTO video_revisions (video_id, revision_number, revision_kind, ruleset_id)
    SELECT new_video.id, 1, 'media', old_ruleset.id FROM new_video, old_ruleset
    RETURNING id
),
title_revision AS (
    INSERT INTO video_revisions (video_id, revision_number, revision_kind, ruleset_id)
    SELECT new_video.id, 1, 'title', new_ruleset.id FROM new_video, new_ruleset
    RETURNING id
),
new_token AS (
    INSERT INTO api_tokens (user_id, name, token_hash)
    SELECT id, 'smoke test', '\x${TOKEN_DIGEST}'::bytea FROM new_user
    RETURNING id
)
SELECT new_user.id, new_video.id,
       (SELECT version FROM rulesets JOIN old_ruleset ON old_ruleset.id = rulesets.id),
       (SELECT version FROM rulesets JOIN new_ruleset ON new_ruleset.id = rulesets.id),
       (SELECT code FROM rules WHERE ruleset_id = (SELECT id FROM new_ruleset)
          AND code NOT IN (SELECT code FROM rules WHERE ruleset_id = (SELECT id FROM old_ruleset))
        LIMIT 1)
FROM new_user, new_video, media_revision, title_revision, new_token
SQL
)"

IFS='|' read -r user_id video_id old_version new_version newer_rule_code <<<"${fixture}"

if [[ -z "${newer_rule_code}" ]]; then
    echo "FAIL the database has no rule that exists only in the newer ruleset;" \
         "the non-retroactivity assertions would be vacuous. Run 'make db-seed'." >&2
    exit 1
fi

echo "Fixture: video ${video_id}, published under ${old_version}, title edited under ${new_version}"
echo "Testing that ${newer_rule_code} (introduced in ${new_version}) cannot reach the ${old_version} media"
echo

# ---------------------------------------------------------------------------
# Start the API.
# ---------------------------------------------------------------------------

echo "Building the API..."
(cd "${REPOSITORY_ROOT}/services/api" && go build -o "${work_directory}/rivo-api" ./cmd/server)

HTTP_ADDRESS="127.0.0.1:${API_PORT}" \
DATABASE_URL="${DATABASE_URL}" \
VIDEO_STORAGE_PATH="${work_directory}/videos" \
MAX_UPLOAD_BYTES=1048576 \
UPLOAD_RATE_LIMIT_BURST=100 \
UPLOAD_RATE_LIMIT_PER_MINUTE=600 \
    "${work_directory}/rivo-api" >"${work_directory}/api.log" 2>&1 &
api_pid=$!

for _ in $(seq 1 50); do
    if curl -sf "${API_URL}/healthz" >/dev/null 2>&1; then
        break
    fi
    if ! kill -0 "${api_pid}" 2>/dev/null; then
        echo "the API exited during startup:" >&2
        cat "${work_directory}/api.log" >&2
        exit 1
    fi
    sleep 0.2
done

echo "Running assertions..."
echo

# ---------------------------------------------------------------------------
# Health and authentication.
# ---------------------------------------------------------------------------

expect_response GET /healthz 200 "health needs no credential"

expect_response POST /api/reports 401 "reporting anonymously is refused" \
    -d "{\"videoId\":\"${video_id}\",\"ruleCode\":\"R-4\"}"

expect_response POST /api/reports 401 "an unknown token is refused" \
    -H "Authorization: Bearer not-a-real-token" \
    -d "{\"videoId\":\"${video_id}\",\"ruleCode\":\"R-4\"}"

# ---------------------------------------------------------------------------
# Non-retroactive enforcement, over HTTP, against real data.
# ---------------------------------------------------------------------------

expect_response POST /api/reports 201 "a rule introduced later is recorded as rejected" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"videoId\":\"${video_id}\",\"ruleCode\":\"${newer_rule_code}\"}"
expect_body_contains '"status":"rejected_not_applicable"' "the rejection names its status"
expect_body_contains "\"rulesetVersion\":\"${old_version}\"" "the media is judged under ${old_version}"
expect_body_contains '"reason":' "the rejection carries a reason"

expect_response POST /api/reports 201 "the same rule against the newer title revision is queued" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"videoId\":\"${video_id}\",\"revisionKind\":\"title\",\"ruleCode\":\"${newer_rule_code}\"}"
expect_body_contains '"status":"queued_for_review"' "the title report is queued"
expect_body_contains "\"rulesetVersion\":\"${new_version}\"" "the title is judged under ${new_version}"

expect_response POST /api/reports 409 "the same allegation cannot be filed twice" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"videoId\":\"${video_id}\",\"revisionKind\":\"title\",\"ruleCode\":\"${newer_rule_code}\"}"

expect_response POST /api/reports 400 "a client-supplied ruleset is refused" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"videoId\":\"${video_id}\",\"ruleCode\":\"${newer_rule_code}\",\"rulesetId\":\"forged\"}"

expect_response POST /api/reports 404 "an unknown video is not found" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"videoId":"00000000-0000-4000-8000-000000000000","ruleCode":"R-4"}'

# ---------------------------------------------------------------------------
# Uploads.
# ---------------------------------------------------------------------------

printf '\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2avc1mp41' >"${work_directory}/clip.mp4"
head -c 100000 /dev/urandom >>"${work_directory}/clip.mp4"
printf '#!/bin/sh\necho not a video\n' >"${work_directory}/payload.mp4"

# Genuinely an MP4, so it reaches the size limit rather than being turned away by
# container detection first.
printf '\x00\x00\x00\x20ftypisom\x00\x00\x02\x00isomiso2avc1mp41' >"${work_directory}/oversized.mp4"
head -c 2000000 /dev/urandom >>"${work_directory}/oversized.mp4"

expect_response POST /api/videos 401 "uploading anonymously is refused" \
    -F "video=@${work_directory}/clip.mp4"

expect_response POST /api/videos 201 "an authenticated upload is stored" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -F "video=@${work_directory}/clip.mp4"
expect_body_contains '"contentType":"video/mp4"' "the container is detected from the bytes"

expect_response POST /api/videos 415 "a script named .mp4 is refused" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -F "video=@${work_directory}/payload.mp4"

expect_response POST /api/videos 413 "an upload over the limit is refused" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -F "video=@${work_directory}/oversized.mp4"

stored_count="$(find "${work_directory}/videos" -name source -type f 2>/dev/null | wc -l)"
if [[ "${stored_count}" == "1" ]]; then
    pass "only the accepted upload reached storage"
else
    fail "only the accepted upload reached storage (found ${stored_count} stored files)"
fi

leftover_directories="$(find "${work_directory}/videos" -mindepth 2 -maxdepth 2 -type d -empty 2>/dev/null | wc -l)"
if [[ "${leftover_directories}" == "0" ]]; then
    pass "refused uploads left no empty directories behind"
else
    fail "refused uploads left ${leftover_directories} empty directories behind"
fi

# ---------------------------------------------------------------------------
# Every decision above must have left an audit trail.
# ---------------------------------------------------------------------------

audit_count="$(psql "${DATABASE_URL}" -qtA -c "
    SELECT count(*) FROM audit_events WHERE actor_user_id = '${user_id}'")"
if [[ "${audit_count}" == "2" ]]; then
    pass "both report decisions were audited"
else
    fail "both report decisions were audited (found ${audit_count} events)"
fi

# ---------------------------------------------------------------------------
# Shutdown.
# ---------------------------------------------------------------------------

kill -TERM "${api_pid}"
shutdown_deadline=$((SECONDS + 15))
while kill -0 "${api_pid}" 2>/dev/null && ((SECONDS < shutdown_deadline)); do
    sleep 0.2
done

if kill -0 "${api_pid}" 2>/dev/null; then
    fail "the API drains and exits on SIGTERM"
else
    pass "the API drains and exits on SIGTERM"
    api_pid=""
fi

echo
if ((failures > 0)); then
    echo "${failures} assertion(s) failed"
    echo "--- API log ---"
    cat "${work_directory}/api.log"
    exit 1
fi

echo "All assertions passed"
