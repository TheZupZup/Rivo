# Non-retroactive enforcement

## Policy

A creator must not receive a platform-policy strike, suspension, or ban for an action that complied with the rules applicable when that action occurred.

A newly introduced rule applies only from its effective date forward.

## How the product enforces this

Every content revision records the ruleset that applied when the revision was created.

When a report names a rule, the API checks whether the referenced rule exists in the revision's ruleset.

If the rule did not exist, that allegation is rejected before a moderation case is created.

The report may still be evaluated under another rule that genuinely applied at the time, but the new rule cannot be applied retroactively.

## Example

- Video uploaded: 2026-09-10
- Applicable ruleset: v1.3
- Rule R-17 introduced: 2027-02-01 in v2.0
- User reports the 2026 upload for R-17

Result: `rejected_not_applicable`

No creator-policy strike is created from R-17.

## Legal removals

Legal obligations are separate from creator-policy enforcement.

If the platform is legally required to remove content, that removal is recorded as a legal event. It does not automatically imply that the creator violated the platform rules that existed at publication time.
