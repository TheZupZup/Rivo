package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	"github.com/TheZupZup/Rivo/services/api/internal/platform"
)

type stubRevisionStore struct{}

func (stubRevisionStore) LatestRevision(_ context.Context, videoID, revisionKind string) (platform.ContentRevision, error) {
	if videoID != "video-2026" {
		return platform.ContentRevision{}, platform.ErrRevisionNotFound
	}

	return platform.ContentRevision{
		ID:             "revision-1",
		VideoID:        videoID,
		Kind:           revisionKind,
		RulesetID:      "ruleset-v1.3",
		RulesetVersion: "v1.3",
	}, nil
}

type stubRuleStore struct{}

func (stubRuleStore) RuleExistsInRuleset(_ context.Context, _, ruleCode string) (bool, error) {
	return ruleCode == "R-4", nil
}

type stubReportStore struct {
	err      error
	received platform.ReportRecord
}

func (store *stubReportStore) CreateReport(_ context.Context, record platform.ReportRecord) (string, error) {
	if store.err != nil {
		return "", store.err
	}

	store.received = record
	return "report-1", nil
}

func newTestReportHandler(reports *stubReportStore) ReportHandler {
	return NewReportHandler(platform.NewReportService(
		stubRevisionStore{},
		reports,
		platform.NewReportEligibilityService(stubRuleStore{}),
	))
}

func submitReport(t *testing.T, handler ReportHandler, body string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/reports", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authenticated {
		request = request.WithContext(auth.WithIdentity(request.Context(), auth.Identity{UserID: "user-1"}))
	}

	response := httptest.NewRecorder()
	handler.Submit(response, request)

	return response
}

func TestSubmitReportRecordsAnApplicableAllegation(t *testing.T) {
	reports := &stubReportStore{}
	response := submitReport(t, newTestReportHandler(reports), `{"videoId":"video-2026","ruleCode":"R-4"}`, true)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	var payload platform.SubmittedReport
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != platform.ReportStatusQueuedForReview {
		t.Fatalf("expected the report to be queued, got %q", payload.Status)
	}
	if payload.RulesetVersion != "v1.3" {
		t.Fatalf("expected the response to name the ruleset judged, got %q", payload.RulesetVersion)
	}
}

func TestSubmitReportStillRecordsAnInapplicableAllegation(t *testing.T) {
	reports := &stubReportStore{}
	response := submitReport(t, newTestReportHandler(reports), `{"videoId":"video-2026","ruleCode":"R-17"}`, true)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
	if reports.received.Status != platform.ReportStatusRejected {
		t.Fatalf("expected a rejected record, got %q", reports.received.Status)
	}
	if reports.received.RejectionReason == "" {
		t.Fatal("expected a reason the reporter and the creator can both read")
	}
}

func TestSubmitReportTakesTheReporterFromTheTokenNotTheBody(t *testing.T) {
	reports := &stubReportStore{}
	handler := newTestReportHandler(reports)

	// A client that tries to file a report as somebody else is rejected outright
	// rather than silently attributed to itself.
	response := submitReport(t, handler, `{"videoId":"video-2026","ruleCode":"R-4","reporterUserId":"user-2"}`, true)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected unknown fields to be refused, got %d: %s", response.Code, response.Body.String())
	}

	response = submitReport(t, handler, `{"videoId":"video-2026","ruleCode":"R-4"}`, true)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.Code)
	}
	if reports.received.ReporterUserID != "user-1" {
		t.Fatalf("expected the authenticated user, got %q", reports.received.ReporterUserID)
	}
}

func TestSubmitReportRejectsAClientChosenRuleset(t *testing.T) {
	// The ruleset is frozen on the revision. Accepting one from the browser would
	// let a modified client pick the rules a creator is judged under.
	response := submitReport(t, newTestReportHandler(&stubReportStore{}),
		`{"videoId":"video-2026","ruleCode":"R-17","rulesetId":"ruleset-v2.0"}`, true)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSubmitReportRequiresAnIdentity(t *testing.T) {
	response := submitReport(t, newTestReportHandler(&stubReportStore{}), `{"videoId":"video-2026","ruleCode":"R-4"}`, false)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestSubmitReportMapsDomainErrorsToStatusCodes(t *testing.T) {
	cases := map[string]struct {
		body     string
		reports  *stubReportStore
		expected int
	}{
		"unknown video":       {body: `{"videoId":"video-missing","ruleCode":"R-4"}`, reports: &stubReportStore{}, expected: http.StatusNotFound},
		"missing video":       {body: `{"ruleCode":"R-4"}`, reports: &stubReportStore{}, expected: http.StatusBadRequest},
		"missing rule":        {body: `{"videoId":"video-2026"}`, reports: &stubReportStore{}, expected: http.StatusBadRequest},
		"unknown kind":        {body: `{"videoId":"video-2026","ruleCode":"R-4","revisionKind":"transcript"}`, reports: &stubReportStore{}, expected: http.StatusBadRequest},
		"malformed json":      {body: `not json`, reports: &stubReportStore{}, expected: http.StatusBadRequest},
		"duplicate report":    {body: `{"videoId":"video-2026","ruleCode":"R-4"}`, reports: &stubReportStore{err: platform.ErrDuplicateReport}, expected: http.StatusConflict},
		"storage unavailable": {body: `{"videoId":"video-2026","ruleCode":"R-4"}`, reports: &stubReportStore{err: context.DeadlineExceeded}, expected: http.StatusInternalServerError},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			response := submitReport(t, newTestReportHandler(testCase.reports), testCase.body, true)
			if response.Code != testCase.expected {
				t.Fatalf("expected %d, got %d: %s", testCase.expected, response.Code, response.Body.String())
			}
		})
	}
}
