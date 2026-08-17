package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/TheZupZup/Rivo/services/api/internal/auth"
	"github.com/TheZupZup/Rivo/services/api/internal/platform"
)

// maxReportBodyBytes is generous for the four short fields a report carries.
const maxReportBodyBytes int64 = 8 * 1024

type ReportHandler struct {
	reportService platform.ReportService
}

func NewReportHandler(reportService platform.ReportService) ReportHandler {
	return ReportHandler{reportService: reportService}
}

// reportRequest is deliberately narrow.
//
// The client names what it is looking at and which rule it believes was broken. It
// does not name the reporter (that comes from the token), the revision (resolved
// server-side) or the ruleset (frozen on the revision). Accepting any of those from
// the browser would let a modified client pick the rules it is judged under.
type reportRequest struct {
	VideoID      string `json:"videoId"`
	RevisionKind string `json:"revisionKind"`
	RuleCode     string `json:"ruleCode"`
}

func (handler ReportHandler) Submit(w http.ResponseWriter, request *http.Request) {
	identity, ok := auth.IdentityFrom(request.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "a valid API token is required")
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, maxReportBodyBytes)

	var body reportRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "request body must be a JSON object with videoId, ruleCode and an optional revisionKind")
		return
	}

	submitted, err := handler.reportService.Submit(request.Context(), platform.SubmitReportCommand{
		ReporterUserID: identity.UserID,
		VideoID:        body.VideoID,
		RevisionKind:   body.RevisionKind,
		RuleCode:       body.RuleCode,
	})
	if err != nil {
		writeReportError(w, err)
		return
	}

	// A rejected report is still created and still audited: a report is not a
	// conviction, and refusing to record one would make the rejection unreviewable.
	WriteJSON(w, http.StatusCreated, submitted)
}

func writeReportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, platform.ErrRevisionNotFound):
		WriteError(w, http.StatusNotFound, "no such video revision")
	case errors.Is(err, platform.ErrDuplicateReport):
		WriteError(w, http.StatusConflict, "you have already reported this revision for this rule")
	case errors.Is(err, platform.ErrUnknownVideo),
		errors.Is(err, platform.ErrUnknownRuleCode),
		errors.Is(err, platform.ErrInvalidRevision):
		WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, platform.ErrUnknownReporter):
		WriteError(w, http.StatusUnauthorized, "a valid API token is required")
	default:
		log.Printf("submit report failed: %v", err)
		WriteError(w, http.StatusInternalServerError, "could not record this report")
	}
}
