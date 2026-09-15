## Conflict Detection Report

### BLOCKERS (0)

None. No ADR documents were present in the ingest set (LOCKED-vs-LOCKED and LOCKED-vs-existing-context checks are not applicable), no cross-reference cycles reached the traversal-hazard threshold (see INFO below), and no UNKNOWN-confidence-low classifications were found.

### WARNINGS (3)

[WARNING] Competing variants: roll mechanic (dice vs. spin wheel)
  Found: docs/original-concept-notes.md describes "a dice-roll randomizer" / "a roll assigns you a category" (dice framing, per its own title "daily dice-roll taste log"). README.md states "Spin wheel, not a dice roll" as a resolved decision, with rationale (wedge sizing maps to reweighting, no separate weighting UI needed, deceleration moment is more shareable, a die caps out around 6 faces).
  Impact: Both are DOC-type sources with no precedence over each other, so synthesis cannot auto-pick a winner. README.md does frame itself as the later, decision-bearing document and explicitly labels docs/original-concept-notes.md as "pre-research, pre-decisions" — but that self-declared framing is not a precedence rule this pipeline applies automatically.
  → Confirm "spin wheel" is the intended current mechanic before downstream planning locks it in; if confirmed, consider superseding/archiving the dice framing in docs/original-concept-notes.md or promoting the wheel choice to a formal ADR.

[WARNING] Competing variants: roll cadence (once-a-day-or-on-demand vs. strictly once-a-day)
  Found: docs/original-concept-notes.md states "Once a day (or on demand)." README.md states "strictly once a day" with no unlimited manual re-roll, optionally earning one extra reroll token per week via streak milestones — listed under "Changes to the original mechanics (from prior-art research)."
  Impact: Same DOC-vs-DOC precedence gap as above; RESEARCH.md's Risks section independently corroborates the README direction ("Evidence leans toward strict daily scarcity over 'once a day or on demand'"), but corroboration by a third DOC does not itself establish precedence in this pipeline's rules.
  → Confirm "strictly once a day + optional streak-earned reroll token" is the intended current cadence before downstream planning locks it in.

[WARNING] Competing variants: photo-source / authenticity rule
  Found: docs/original-concept-notes.md requires "No auto-fetched/stock images, ever... a photo you actually took... never a pulled-from-a-database poster or cover" (strict self-taken rule, no explicit live-vs-library distinction). README.md's "Decisions made so far" table allows "Camera roll / photo library is allowed, not camera-only," and explicitly flags that this "reopens part of the authenticity-enforcement question since there's no technical guarantee against posting stock/official art — accepted tradeoff, not solved yet." RESEARCH.md's Risks section independently flags the same unresolved enforcement gap.
  Impact: This is the highest-stakes divergence of the three — it touches the product's core authenticity claim, which RESEARCH.md's Risks section calls out as the entire value proposition in comparable products (Poparazzi's authenticity-by-constraint collapse). All three sources agree this specific tension is currently unresolved, but they do not agree on what the rule actually is (strict self-taken-only vs. camera-roll-allowed-with-caveat).
  → Decide and record explicitly whether camera-roll uploads are in scope, and whether/how non-self-taken (stock/official art) images will be technically or socially discouraged, before this becomes a formal requirement.

### INFO (1)

[INFO] Mutual cross-reference detected: README.md <-> RESEARCH.md
  Note: README.md's cross_refs list ./RESEARCH.md, and RESEARCH.md's cross_refs list README.md (a 2-node cycle in the classification cross-ref graph, using inconsistent relative path forms typical of hand-written "see also" links rather than a crafted dependency chain). This was not treated as a BLOCKER: both sources are DOC-type with no supersedes/dependency semantics, and this synthesis pass extracts each classified doc independently rather than traversing cross_refs, so no synthesis loop can occur. Both documents were synthesized normally into context.md.
