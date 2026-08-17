# Feasibility assessment

This document records an honest reading of what Rivo is, what it is not, and what
would have to be true for it to survive. It is written to be revised: every claim
here is a bet, and bets should be re-examined when the evidence changes.

## What Rivo actually is today

Rivo is not a video platform yet. It is a schema plus one storage endpoint.

| Claimed in the README | State in the code |
| --- | --- |
| Accounts and channels | Tables exist, no code path creates or reads them |
| Video metadata | `videos` / `video_revisions` tables exist, uploads do not write to them |
| Versioned community rules | Tables exist, seeded only by the dev seed script |
| Non-retroactive report validation | Domain service and Postgres store exist and are wired to `POST /api/reports` |
| Moderation cases and appeals | Tables exist, no code path yet |
| Immutable audit records | Table, append-only enforcement and report events exist |
| Replaceable storage boundary | Real: `storage.VideoStore` with a local implementation |

The gap between ambition and implementation is the single largest risk. It is not a
coding problem; it is a sequencing problem, and the sequencing recommendation is at
the end of this document.

## The differentiator is real, and it is the only defensible asset

Most "YouTube alternatives" differentiate on features (higher bitrate, better
revenue share, no ads). Those are all things a well-funded incumbent can copy in a
quarter. Rivo differentiates on a *structural* commitment — non-retroactive
enforcement — that an incumbent cannot copy without admitting its current practice
was unfair.

That commitment is genuinely encoded here rather than merely promised:

- `video_revisions.ruleset_id` freezes the rules that applied when a revision was created.
- `rules` are unique per `(ruleset_id, code)`, so a rule code is meaningless without its ruleset.
- `moderation_cases` is constrained so that its `applicable_ruleset_id` must equal the ruleset of the revision it judges (migration `002`).
- `audit_events` is append-only at the privilege level, not merely by convention.

A moderator cannot quietly apply a 2027 rule to a 2026 upload, because the database
will refuse the write. That is the product.

## Regulatory position: an asset, not just a cost

Rivo's moderation model maps closely onto the EU Digital Services Act:

| DSA obligation | Rivo construct |
| --- | --- |
| Art. 16 — notice and action | `reports` with an explicit status machine |
| Art. 17 — statement of reasons | `moderation_cases.decision_reason`, `evidence_reference` |
| Art. 20 — internal complaint handling | `appeals` |
| Art. 24 — transparency reporting | `audit_events` as the reporting substrate |
| Separation of legal orders from policy enforcement | `legal_removals` distinct from `moderation_cases` |

This should be stated loudly. "Compliance-native moderation" is a more credible
reason for a creator or an institution to adopt Rivo than "we are nicer".

The obligations that are *not* addressed and that gate any public launch:

- **CSAM detection.** Non-negotiable, legally and morally, from the first public upload. Requires hash-matching against a recognised database and a reporting pipeline to the relevant authority. There is no lawful "we will add it later" launch.
- **Copyright.** No notice-and-staydown, no fingerprinting. This is where video platforms die: rightsholder pressure scales faster than a volunteer moderation team.
- **Age assurance** for age-restricted content, now mandated in several jurisdictions.
- **Trusted flagger** intake (DSA art. 22) if EU users are served at scale.

## Economics: the real wall

Storage is cheap and getting cheaper. Egress is not. A video platform's cost curve
is dominated by bandwidth, and bandwidth cost is roughly linear in success — the
more the product works, the more it costs, with no natural economy of scale until
you own peering.

Compounding this, Rivo's moderation design is deliberately *expensive*:

- `moderation_reviews` supports `initial` and `second_review` stages, i.e. two humans on contested cases.
- Appeals imply a third human.
- Explainability implies written reasons, not a template.

That is the right design ethically and the wrong design for a free, ad-funded, open
signup platform. The two can only be reconciled by narrowing who can publish.

**Conclusion: Rivo is feasible as a niche, high-trust, invite-or-subscription
platform. It is not feasible as a general-purpose YouTube competitor**, and the
README should stop implying otherwise. Plausible viable shapes:

1. **Federated / self-hosted instances.** Each operator bears its own bandwidth and moderation cost; Rivo ships the software and the governance model. The AGPL choice already points here.
2. **Vertical platform.** One domain (education, research, public institutions, a professional body) where the audience is bounded, the moderation load is low, and auditability is a purchasing requirement rather than a nice-to-have.
3. **Moderation infrastructure.** The rulesets / revisions / cases / appeals / audit core is useful to *other* platforms. It may be worth more as a component than as a destination site.

Option 2 is the one that most directly monetises the work already done.

## Design issue found during review: metadata edits and the invariant

`video_revisions` binds a ruleset to each revision *kind* (`media`, `title`,
`description`, `thumbnail`). A creator who fixes a typo in a 2026 title during 2027
creates a `title` revision bound to the 2027 ruleset.

This is defensible — editing a title in 2027 genuinely is an act performed under the
2027 rules — but it becomes a violation of the README's promise the moment a case
opened against that title revision is used to sanction the *media* published in
2026.

The fix applied here is to make enforcement scope explicit rather than implicit:
`moderation_cases` now names the exact `video_revision_id` it judges, and a database
constraint forces `applicable_ruleset_id` to be that revision's ruleset. A sanction
therefore always has a scope that is inspectable after the fact, and "which content
was actually judged" is never inferred.

**Open policy question, deliberately not answered in code:** should a strike against
a metadata revision count toward the same creator-strike total as a strike against
media? The product should probably say no, and the schema is ready to distinguish
them when the strike model is built.

## Recommended sequencing

The current milestone plan puts playback and transcoding next. That is the wrong
order. The differentiator is the moderation core, and until recently none of it
executed — it was documentation with tables behind it.

1. **Make moderation real.** (Started here: Postgres wired, reports evaluated and persisted, audit events written.) Continue with cases, reviews, appeals, and strikes.
2. **Make identity real.** Token auth exists as a minimum viable mechanism; it needs a real signup, session and channel-ownership model before anything is public.
3. **Then** playback and transcoding, because a platform with no viewing is not testable by users.
4. **Only then** consider public signups — and not before CSAM scanning and a copyright process exist.

## Things that should be decided before writing more code

- Who is allowed to publish? The answer determines the cost model and therefore whether the project survives.
- Is the target a hosted service, a federation of instances, or a library? The README currently implies the first, the licence implies the second, and the architecture is best suited to the third.
- What is the strike model? `moderation_cases.decision` enumerates outcomes but nothing accumulates them, and accumulation is where non-retroactivity gets hard.
