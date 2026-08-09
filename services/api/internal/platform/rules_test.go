package platform

import (
	"context"
	"errors"
	"testing"
)

type fakeRuleStore struct {
	rules map[string]map[string]bool
	err   error
}

func (store fakeRuleStore) RuleExistsInRuleset(_ context.Context, rulesetID, ruleCode string) (bool, error) {
	if store.err != nil {
		return false, store.err
	}

	return store.rules[rulesetID][ruleCode], nil
}

func TestReportEligibilityRejectsRuleThatDidNotApply(t *testing.T) {
	service := NewReportEligibilityService(fakeRuleStore{
		rules: map[string]map[string]bool{
			"ruleset-v1.3": {"R-4": true},
		},
	})

	result, err := service.Evaluate(context.Background(), ContentRevision{
		RulesetID:      "ruleset-v1.3",
		RulesetVersion: "v1.3",
	}, "R-17")
	if err != nil {
		t.Fatalf("evaluate report eligibility: %v", err)
	}

	if result.Eligible {
		t.Fatal("expected report to be rejected because the rule did not apply to the revision")
	}
}

func TestReportEligibilityAcceptsApplicableRule(t *testing.T) {
	service := NewReportEligibilityService(fakeRuleStore{
		rules: map[string]map[string]bool{
			"ruleset-v1.3": {"R-4": true},
		},
	})

	result, err := service.Evaluate(context.Background(), ContentRevision{
		RulesetID:      "ruleset-v1.3",
		RulesetVersion: "v1.3",
	}, "R-4")
	if err != nil {
		t.Fatalf("evaluate report eligibility: %v", err)
	}

	if !result.Eligible {
		t.Fatalf("expected report to be eligible, got reason %q", result.Reason)
	}
}

func TestReportEligibilityPropagatesLookupFailure(t *testing.T) {
	lookupFailure := errors.New("database unavailable")
	service := NewReportEligibilityService(fakeRuleStore{err: lookupFailure})

	_, err := service.Evaluate(context.Background(), ContentRevision{
		RulesetID:      "ruleset-v1.3",
		RulesetVersion: "v1.3",
	}, "R-4")

	if !errors.Is(err, ErrRuleLookup) {
		t.Fatalf("expected ErrRuleLookup, got %v", err)
	}
	if !errors.Is(err, lookupFailure) {
		t.Fatalf("expected underlying lookup error, got %v", err)
	}
}
