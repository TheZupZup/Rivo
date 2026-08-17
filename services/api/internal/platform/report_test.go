package platform

import (
	"context"
	"errors"
	"testing"
)

type fakeRevisionStore struct {
	revisions map[string]ContentRevision
}

func (store fakeRevisionStore) LatestRevision(_ context.Context, videoID, revisionKind string) (ContentRevision, error) {
	revision, found := store.revisions[videoID+"/"+revisionKind]
	if !found {
		return ContentRevision{}, ErrRevisionNotFound
	}

	return revision, nil
}

type fakeReportStore struct {
	created []ReportRecord
	err     error
}

func (store *fakeReportStore) CreateReport(_ context.Context, record ReportRecord) (string, error) {
	if store.err != nil {
		return "", store.err
	}

	store.created = append(store.created, record)
	return "report-1", nil
}

func newTestReportService(reports *fakeReportStore) ReportService {
	revisions := fakeRevisionStore{revisions: map[string]ContentRevision{
		"video-2026/media": {
			ID:             "revision-media",
			VideoID:        "video-2026",
			Kind:           "media",
			RulesetID:      "ruleset-v1.3",
			RulesetVersion: "v1.3",
		},
		"video-2026/title": {
			ID:             "revision-title",
			VideoID:        "video-2026",
			Kind:           "title",
			RulesetID:      "ruleset-v2.0",
			RulesetVersion: "v2.0",
		},
	}}

	rules := fakeRuleStore{rules: map[string]map[string]bool{
		"ruleset-v1.3": {"R-4": true},
		"ruleset-v2.0": {"R-4": true, "R-17": true},
	}}

	return NewReportService(revisions, reports, NewReportEligibilityService(rules))
}

func TestSubmitRejectsRuleIntroducedAfterThePublishedRevision(t *testing.T) {
	reports := &fakeReportStore{}
	service := newTestReportService(reports)

	submitted, err := service.Submit(context.Background(), SubmitReportCommand{
		ReporterUserID: "user-1",
		VideoID:        "video-2026",
		RuleCode:       "R-17",
	})
	if err != nil {
		t.Fatalf("submit report: %v", err)
	}

	if submitted.Status != ReportStatusRejected {
		t.Fatalf("expected %q, got %q", ReportStatusRejected, submitted.Status)
	}
	if submitted.RulesetVersion != "v1.3" {
		t.Fatalf("expected the report to be judged under v1.3, got %q", submitted.RulesetVersion)
	}
	if submitted.Reason == "" {
		t.Fatal("expected a rejection reason a creator can be shown")
	}

	// The rejection is still recorded: a report is not a conviction, but an
	// unrecorded rejection cannot be reviewed or appealed.
	if len(reports.created) != 1 {
		t.Fatalf("expected the rejected report to be persisted, got %d records", len(reports.created))
	}
	if reports.created[0].RejectionReason == "" {
		t.Fatal("expected the persisted record to carry the rejection reason")
	}
}

func TestSubmitQueuesReportForRuleThatApplied(t *testing.T) {
	reports := &fakeReportStore{}
	service := newTestReportService(reports)

	submitted, err := service.Submit(context.Background(), SubmitReportCommand{
		ReporterUserID: "user-1",
		VideoID:        "video-2026",
		RuleCode:       "R-4",
	})
	if err != nil {
		t.Fatalf("submit report: %v", err)
	}

	if submitted.Status != ReportStatusQueuedForReview {
		t.Fatalf("expected %q, got %q", ReportStatusQueuedForReview, submitted.Status)
	}
	if reports.created[0].RejectionReason != "" {
		t.Fatalf("expected no rejection reason, got %q", reports.created[0].RejectionReason)
	}
}

// A newer rule may legitimately be applied to metadata edited after that rule
// came into force, but only ever to the revision that was actually edited.
func TestSubmitJudgesMetadataRevisionUnderItsOwnRuleset(t *testing.T) {
	reports := &fakeReportStore{}
	service := newTestReportService(reports)

	submitted, err := service.Submit(context.Background(), SubmitReportCommand{
		ReporterUserID: "user-1",
		VideoID:        "video-2026",
		RevisionKind:   "title",
		RuleCode:       "R-17",
	})
	if err != nil {
		t.Fatalf("submit report: %v", err)
	}

	if submitted.Status != ReportStatusQueuedForReview {
		t.Fatalf("expected %q, got %q", ReportStatusQueuedForReview, submitted.Status)
	}
	if reports.created[0].RevisionID != "revision-title" {
		t.Fatalf("expected the case to be scoped to the title revision, got %q", reports.created[0].RevisionID)
	}
}

func TestSubmitDefaultsToTheMediaRevision(t *testing.T) {
	reports := &fakeReportStore{}
	service := newTestReportService(reports)

	if _, err := service.Submit(context.Background(), SubmitReportCommand{
		ReporterUserID: "user-1",
		VideoID:        "video-2026",
		RuleCode:       "R-4",
	}); err != nil {
		t.Fatalf("submit report: %v", err)
	}

	if reports.created[0].RevisionID != "revision-media" {
		t.Fatalf("expected the media revision by default, got %q", reports.created[0].RevisionID)
	}
}

func TestSubmitValidatesItsInput(t *testing.T) {
	cases := map[string]struct {
		command  SubmitReportCommand
		expected error
	}{
		"missing reporter": {
			command:  SubmitReportCommand{VideoID: "video-2026", RuleCode: "R-4"},
			expected: ErrUnknownReporter,
		},
		"missing video": {
			command:  SubmitReportCommand{ReporterUserID: "user-1", RuleCode: "R-4"},
			expected: ErrUnknownVideo,
		},
		"missing rule": {
			command:  SubmitReportCommand{ReporterUserID: "user-1", VideoID: "video-2026"},
			expected: ErrUnknownRuleCode,
		},
		"unknown revision kind": {
			command: SubmitReportCommand{
				ReporterUserID: "user-1",
				VideoID:        "video-2026",
				RuleCode:       "R-4",
				RevisionKind:   "transcript",
			},
			expected: ErrInvalidRevision,
		},
		"unknown video": {
			command: SubmitReportCommand{
				ReporterUserID: "user-1",
				VideoID:        "video-missing",
				RuleCode:       "R-4",
			},
			expected: ErrRevisionNotFound,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			reports := &fakeReportStore{}
			service := newTestReportService(reports)

			_, err := service.Submit(context.Background(), testCase.command)
			if !errors.Is(err, testCase.expected) {
				t.Fatalf("expected %v, got %v", testCase.expected, err)
			}
			if len(reports.created) != 0 {
				t.Fatal("expected nothing to be persisted for an invalid command")
			}
		})
	}
}

func TestSubmitPropagatesDuplicateReports(t *testing.T) {
	reports := &fakeReportStore{err: ErrDuplicateReport}
	service := newTestReportService(reports)

	_, err := service.Submit(context.Background(), SubmitReportCommand{
		ReporterUserID: "user-1",
		VideoID:        "video-2026",
		RuleCode:       "R-4",
	})

	if !errors.Is(err, ErrDuplicateReport) {
		t.Fatalf("expected ErrDuplicateReport, got %v", err)
	}
}
