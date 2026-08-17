# Non-retroactive enforcement

## Policy

A creator must not receive a platform-policy strike, suspension, or ban for an action that complied with the rules applicable when that action occurred.

A newly introduced rule applies only from its effective date forward.

## How the product enforces this

Every content revision records the ruleset that applied when the revision was created.

When a report names a rule, the API checks whether the referenced rule exists in the revision's ruleset.

If the rule did not exist, that allegation is rejected before a moderation case is created.

The report may still be evaluated under another rule that genuinely applied at the time, but the new rule cannot be applied retroactively.

This is enforced twice, on purpose:

1. `platform.ReportService` refuses to queue an allegation whose rule was not in the revision's ruleset, and records the rejection with a reason.
2. The database refuses to store a moderation case whose `applicable_ruleset_id` differs from the ruleset frozen on the revision it names. A bug, a migration, a future service or a manual `psql` session cannot produce a case that contradicts this policy.

A rejected report is still stored and still audited. Discarding it would leave the
reporter with no record and the creator with no evidence that an allegation was
raised and dismissed.

## Example

- Video uploaded: 2026-09-10
- Applicable ruleset: v1.3
- Rule R-17 introduced: 2027-02-01 in v2.0
- User reports the 2026 upload for R-17

Result: `rejected_not_applicable`

No creator-policy strike is created from R-17.

## Edited metadata

A creator who edits a title, description or thumbnail creates a new revision, and
that revision is bound to the ruleset in effect at the time of the edit. Correcting
a typo in 2027 therefore does expose the *corrected title* to the 2027 rules. That is
intended: the edit is an act performed in 2027.

What must never follow is a sanction against the media published in 2026. Enforcement
scope is explicit for exactly this reason: a moderation case names the revision it
judges, so "which content was actually judged" is recorded rather than inferred.

An open question the product has not yet answered: whether a strike against a
metadata revision should count toward the same creator-strike total as a strike
against media. The schema can distinguish them; the strike model does not exist yet.

## Legal removals

Legal obligations are separate from creator-policy enforcement.

If the platform is legally required to remove content, that removal is recorded as a legal event. It does not automatically imply that the creator violated the platform rules that existed at publication time.
