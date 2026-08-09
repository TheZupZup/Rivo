package platform

import (
	"context"
	"errors"
)

var ErrRuleLookup = errors.New("rule applicability lookup failed")

type ContentRevision struct {
	RulesetID      string
	RulesetVersion string
}

type RuleApplicabilityStore interface {
	RuleExistsInRuleset(ctx context.Context, rulesetID, ruleCode string) (bool, error)
}

type ReportEligibility struct {
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
}

type ReportEligibilityService struct {
	rules RuleApplicabilityStore
}

func NewReportEligibilityService(rules RuleApplicabilityStore) ReportEligibilityService {
	return ReportEligibilityService{rules: rules}
}

func (service ReportEligibilityService) Evaluate(
	ctx context.Context,
	revision ContentRevision,
	reportedRuleCode string,
) (ReportEligibility, error) {
	exists, err := service.rules.RuleExistsInRuleset(ctx, revision.RulesetID, reportedRuleCode)
	if err != nil {
		return ReportEligibility{}, errors.Join(ErrRuleLookup, err)
	}

	if !exists {
		return ReportEligibility{
			Eligible: false,
			Reason:   "reported rule was not applicable to this content revision",
		}, nil
	}

	return ReportEligibility{Eligible: true}, nil
}
