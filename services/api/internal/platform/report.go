package platform

import (
	"context"
	"errors"
	"fmt"
)

const (
	ReportStatusQueuedForReview = "queued_for_review"
	ReportStatusRejected        = "rejected_not_applicable"
)

// RevisionKindMedia is the default target of a report: the published video itself,
// as opposed to metadata a creator may have edited later under a newer ruleset.
const RevisionKindMedia = "media"

var revisionKinds = map[string]bool{
	RevisionKindMedia: true,
	"title":           true,
	"description":     true,
	"thumbnail":       true,
}

var (
	ErrRevisionNotFound  = errors.New("no such content revision")
	ErrUnknownRuleCode   = errors.New("rule code is required")
	ErrUnknownVideo      = errors.New("video id is required")
	ErrUnknownReporter   = errors.New("reporter is required")
	ErrInvalidRevision   = errors.New("unknown revision kind")
	ErrDuplicateReport   = errors.New("this reporter already filed this report")
	ErrPersistReport     = errors.New("persist report failed")
	ErrResolveRevisionIO = errors.New("content revision lookup failed")
)

// ContentRevisionStore resolves which revision of a video a report is judged
// against. The caller names a video and a revision kind; the store decides which
// concrete revision that is and which ruleset was frozen on it.
type ContentRevisionStore interface {
	LatestRevision(ctx context.Context, videoID, revisionKind string) (ContentRevision, error)
}

// ReportRecord is a report as it will be stored, with its verdict already decided.
type ReportRecord struct {
	ReporterUserID  string
	VideoID         string
	RevisionID      string
	RuleCode        string
	Status          string
	RejectionReason string
}

// ReportStore persists a report together with its audit trail. Implementations must
// write both atomically: a report that exists without an audit event, or an audit
// event without a report, is a moderation history that cannot be trusted.
type ReportStore interface {
	CreateReport(ctx context.Context, record ReportRecord) (string, error)
}

type SubmitReportCommand struct {
	ReporterUserID string
	VideoID        string
	RevisionKind   string
	RuleCode       string
}

type SubmittedReport struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	RulesetVersion string `json:"rulesetVersion"`
	RevisionKind   string `json:"revisionKind"`
	Reason         string `json:"reason,omitempty"`
}

// ReportService turns a user's allegation into a stored report with a verdict on
// whether it may proceed to human review at all.
//
// The non-retroactivity invariant lives here: a rule that did not exist in the
// ruleset frozen on the revision is rejected before any moderation case can be
// created from it.
type ReportService struct {
	revisions   ContentRevisionStore
	reports     ReportStore
	eligibility ReportEligibilityService
}

func NewReportService(
	revisions ContentRevisionStore,
	reports ReportStore,
	eligibility ReportEligibilityService,
) ReportService {
	return ReportService{
		revisions:   revisions,
		reports:     reports,
		eligibility: eligibility,
	}
}

func (service ReportService) Submit(ctx context.Context, command SubmitReportCommand) (SubmittedReport, error) {
	if command.ReporterUserID == "" {
		return SubmittedReport{}, ErrUnknownReporter
	}
	if command.VideoID == "" {
		return SubmittedReport{}, ErrUnknownVideo
	}
	if command.RuleCode == "" {
		return SubmittedReport{}, ErrUnknownRuleCode
	}

	revisionKind := command.RevisionKind
	if revisionKind == "" {
		revisionKind = RevisionKindMedia
	}
	if !revisionKinds[revisionKind] {
		return SubmittedReport{}, fmt.Errorf("%w: %q", ErrInvalidRevision, revisionKind)
	}

	revision, err := service.revisions.LatestRevision(ctx, command.VideoID, revisionKind)
	if err != nil {
		if errors.Is(err, ErrRevisionNotFound) {
			return SubmittedReport{}, err
		}

		return SubmittedReport{}, errors.Join(ErrResolveRevisionIO, err)
	}

	eligibility, err := service.eligibility.Evaluate(ctx, revision, command.RuleCode)
	if err != nil {
		return SubmittedReport{}, err
	}

	record := ReportRecord{
		ReporterUserID: command.ReporterUserID,
		VideoID:        revision.VideoID,
		RevisionID:     revision.ID,
		RuleCode:       command.RuleCode,
		Status:         ReportStatusQueuedForReview,
	}
	if !eligibility.Eligible {
		record.Status = ReportStatusRejected
		record.RejectionReason = eligibility.Reason
	}

	reportID, err := service.reports.CreateReport(ctx, record)
	if err != nil {
		if errors.Is(err, ErrDuplicateReport) {
			return SubmittedReport{}, err
		}

		return SubmittedReport{}, errors.Join(ErrPersistReport, err)
	}

	return SubmittedReport{
		ID:             reportID,
		Status:         record.Status,
		RulesetVersion: revision.RulesetVersion,
		RevisionKind:   revision.Kind,
		Reason:         record.RejectionReason,
	}, nil
}
