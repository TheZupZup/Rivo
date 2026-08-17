package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TheZupZup/Rivo/services/api/internal/platform"
)

// RuleStore answers whether a rule existed in a given ruleset.
//
// Rules are unique per (ruleset_id, code), so a code alone means nothing: asking
// "does R-17 exist" is only meaningful against the ruleset frozen on the revision
// being judged. That is what makes retroactive application impossible here.
type RuleStore struct {
	pool *pgxpool.Pool
}

func NewRuleStore(pool *pgxpool.Pool) RuleStore {
	return RuleStore{pool: pool}
}

func (store RuleStore) RuleExistsInRuleset(ctx context.Context, rulesetID, ruleCode string) (bool, error) {
	if !isUUID(rulesetID) {
		return false, nil
	}

	const query = `SELECT EXISTS (SELECT 1 FROM rules WHERE ruleset_id = $1 AND code = $2)`

	var exists bool
	if err := store.pool.QueryRow(ctx, query, rulesetID, ruleCode).Scan(&exists); err != nil {
		return false, fmt.Errorf("query rule applicability: %w", err)
	}

	return exists, nil
}

// RevisionStore resolves the revision a report is judged against.
type RevisionStore struct {
	pool *pgxpool.Pool
}

func NewRevisionStore(pool *pgxpool.Pool) RevisionStore {
	return RevisionStore{pool: pool}
}

// LatestRevision returns the most recent revision of a kind for a video, with the
// ruleset that was in effect when that revision was created.
func (store RevisionStore) LatestRevision(ctx context.Context, videoID, revisionKind string) (platform.ContentRevision, error) {
	if !isUUID(videoID) {
		return platform.ContentRevision{}, platform.ErrRevisionNotFound
	}

	const query = `
		SELECT video_revisions.id,
		       video_revisions.video_id,
		       video_revisions.revision_kind,
		       video_revisions.ruleset_id,
		       rulesets.version
		FROM video_revisions
		JOIN rulesets ON rulesets.id = video_revisions.ruleset_id
		WHERE video_revisions.video_id = $1
		  AND video_revisions.revision_kind = $2
		ORDER BY video_revisions.revision_number DESC
		LIMIT 1`

	var revision platform.ContentRevision
	err := store.pool.QueryRow(ctx, query, videoID, revisionKind).Scan(
		&revision.ID,
		&revision.VideoID,
		&revision.Kind,
		&revision.RulesetID,
		&revision.RulesetVersion,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return platform.ContentRevision{}, platform.ErrRevisionNotFound
		}

		return platform.ContentRevision{}, fmt.Errorf("query content revision: %w", err)
	}

	return revision, nil
}

// ReportStore persists reports and their audit trail.
type ReportStore struct {
	pool *pgxpool.Pool
}

func NewReportStore(pool *pgxpool.Pool) ReportStore {
	return ReportStore{pool: pool}
}

// CreateReport writes the report and its audit event in one transaction.
//
// The two must not be separable. A stored report with no audit event leaves a
// moderation action nobody can trace; an audit event with no report describes
// something that never happened.
func (store ReportStore) CreateReport(ctx context.Context, record platform.ReportRecord) (string, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin report transaction: %w", err)
	}
	defer func() {
		_ = transaction.Rollback(ctx)
	}()

	const insertReport = `
		INSERT INTO reports (
			reporter_user_id, video_id, video_revision_id, reported_rule_code, status, rejection_reason
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var rejectionReason *string
	if record.RejectionReason != "" {
		rejectionReason = &record.RejectionReason
	}

	var reportID string
	err = transaction.QueryRow(ctx, insertReport,
		record.ReporterUserID,
		record.VideoID,
		record.RevisionID,
		record.RuleCode,
		record.Status,
		rejectionReason,
	).Scan(&reportID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", platform.ErrDuplicateReport
		}

		return "", fmt.Errorf("insert report: %w", err)
	}

	payload, err := json.Marshal(map[string]string{
		"videoId":         record.VideoID,
		"videoRevisionId": record.RevisionID,
		"ruleCode":        record.RuleCode,
		"status":          record.Status,
		"rejectionReason": record.RejectionReason,
	})
	if err != nil {
		return "", fmt.Errorf("encode audit payload: %w", err)
	}

	const insertAuditEvent = `
		INSERT INTO audit_events (actor_user_id, event_type, subject_type, subject_id, payload)
		VALUES ($1, $2, 'report', $3, $4)`

	eventType := "report.queued_for_review"
	if record.Status == platform.ReportStatusRejected {
		eventType = "report.rejected_not_applicable"
	}

	if _, err := transaction.Exec(ctx, insertAuditEvent, record.ReporterUserID, eventType, reportID, payload); err != nil {
		return "", fmt.Errorf("insert audit event: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit report transaction: %w", err)
	}

	return reportID, nil
}
