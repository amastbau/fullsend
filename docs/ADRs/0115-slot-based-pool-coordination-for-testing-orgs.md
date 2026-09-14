---
title: "115. Slot-based pool coordination for testing orgs"
status: Accepted
supersedes:
  - "0040"
relates_to:
  - agent-infrastructure
topics:
  - testing
  - concurrency
  - ci
---

# 115. Slot-based pool coordination for testing orgs

Date: 2026-09-14

## Status

Accepted — supersedes the locking mechanism in
[ADR 0040](0040-org-pool-for-parallel-e2e-tests.md).

The **coordination primitive** below is the accepted target. As of this writing
the implementation still uses ADR 0040's **exclusive single-lock** pool (one
`e2e-lock` repo per org, one run per org, across the `halfsend-01`…`halfsend-12`
orgs); the slot-based, multiple-runs-per-org mechanism is not yet built. The
automated provisioning tooling (below) *is* implemented. Implementation status
is noted inline per section.

## Context

[ADR 0114](0114-ephemeral-repo-lifecycle-for-testing.md) establishes ephemeral
repos as the testing model. Ephemeral repos have unique names, so concurrent
runs in the same org do not collide on shared state — but they share the org's
rate-limit budget, and how that budget scales differs fundamentally by forge:

- **GitHub.** The rate limit is **org-scoped** and repo-count-scaled (up to
  12,500/hr). At ~3,000 API calls per run, a well-populated org supports roughly
  four concurrent runs. Capacity scales by **adding orgs**.

- **GitLab.com Free.** The rate limit is **per user**, not per group, and cannot
  be raised by adding groups, repos, or tokens for the same user. There is also
  **no API path to add users**: regular user creation (`POST /users`) requires
  instance admin, which SaaS customers do not have, and **service accounts are a
  Premium/Ultimate feature unavailable on Free**. Consequently GitLab.com Free
  concurrency is **hard-capped at a single bot user's budget** and cannot be
  pooled for scale.

ADR 0040's exclusive locking model (one run per org) does not fit GitHub: it
wastes org capacity by blocking concurrent runs that could safely share the rate
limit. The coordination mechanism must:

- Enforce a per-org concurrency cap (not exclusive access) where the forge
  permits concurrency.
- Use an atomic primitive — no read-then-write races.
- Work across both GitHub and GitLab.

## Decision

### Slot-based concurrency control (target; not yet implemented)

Each testing org has N **slot repos** (`test-slot-1` … `test-slot-N`). A slot
repo is a lightweight repository used purely as a distributed semaphore.

- **Acquire.** A run iterates over testing orgs and, within each org, tries to
  create slot repos (`test-slot-1`, then `test-slot-2`, …) until one succeeds.
  Repo creation is atomic — the first creator wins, all others get an
  "already exists" error. If all slots in all orgs are occupied, the run polls
  with backoff until a slot opens or a timeout expires.

- **Release.** On test completion (pass or fail), the run deletes the slot repo
  it created. Cleanup is registered via `t.Cleanup` so crashes still attempt
  release.

- **Stale slot recovery.** A slot repo whose `created_at` exceeds a staleness
  threshold (15 minutes) is assumed to belong to a crashed run. Any run may
  delete the stale slot and recreate it; the recreation is itself atomic, so two
  runs racing to reclaim the same stale slot do not conflict. This is the same
  recovery logic the current exclusive lock already implements
  (`staleLockTimeout = 15 * time.Minute` on the `e2e-lock` repo); the slot design
  applies it per slot.

- **Slot count and forge-specific scaling.** N is tuned per org based on the
  rate-limit budget. On **GitHub**, `N = floor(rate_limit_cap /
  calls_per_run)` — with 12,500/hr and ~3,000 calls/run, N = 4 per org, and
  capacity grows by adding orgs (five orgs → 20 concurrent slots). On
  **GitLab.com Free**, this math does not apply: the per-user cap cannot be
  raised, so a single group with a small N is the ceiling — GitLab supports the
  coordination *primitive* (correctness, no double-booking) but **not pooling
  for scale**. Any GitLab concurrency beyond one user's budget would require a
  paid tier (service accounts) or self-managed admin, both out of scope here.

*Current state:* the code still runs the ADR 0040 exclusive lock — a single
`e2e-lock` repo per org (`lockRepo = "e2e-lock"`), one run per org, across 12
`halfsend-NN` orgs. Migrating to `test-slot-1..N` is the outstanding work.

### Automated org provisioning (implemented, GitHub)

The pool is managed by `e2e/pool/manage.ts`, which automates the lifecycle of
adding a new **GitHub** testing org:

1. **Org creation** — via the GitHub API (`POST /user/orgs`). Note this endpoint
   is only available for GitHub Enterprise-managed accounts; on standard
   github.com org creation is a manual step, so this path assumes an
   Enterprise-managed context.

2. **App installation** — GitHub Apps cannot be installed via API; this step uses
   Playwright browser automation to navigate the app installation UI for each
   required app.

3. **Inference provisioning** — configures GCP Workload Identity Federation and
   the token mint for the new org. (The repo-scoped `fullsend inference provision
   <org>/<repo> --project <gcp-project>` command exists and is used by the
   behaviour install driver; wiring it into `manage.ts` is follow-up.)

Commands: `login`, `create-orgs`, `install-apps`, `provision`, and `setup` (runs
the provisioning steps in sequence). Automated login uses stored credentials
with TOTP; interactive login saves browser state for subsequent headless runs.

*GitLab has no equivalent provisioning path on the Free tier* — see Context:
neither user creation nor service accounts are available, so GitLab pool
expansion is not automatable and not offered.

### Forge portability

The slot/lock mechanism depends on two forge operations, both required to reach
GitLab parity:

- **Create repo (atomic, fail-if-exists).** GitHub returns 422, GitLab returns
  409 or 400 "has already been taken". Both must be surfaced as
  `forge.ErrAlreadyExists` and matched by the pool's `isRepoAlreadyExists`
  helper. **Gap:** today `isRepoAlreadyExists` matches only GitHub's
  "already exists"; GitLab's `CreateRepo` does not map the duplicate error to
  `ErrAlreadyExists` (the "has already been taken" match exists only for CI
  variables/branches/files, not repo creation). This must be closed for GitLab.

- **Read repo metadata (`created_at`)** — used for stale slot detection.
  Available on both GitHub and GitLab project APIs.

No forge-specific external coordinators (GCS, Redis) are required.

## Consequences

- On GitHub, multiple runs share a testing org concurrently up to the slot cap;
  org capacity is used efficiently without risking rate-limit exhaustion, and
  adding capacity is an operational task (provision a new org, append it to the
  pool list).
- The slot count is a hard cap enforced by the number of slot repo names, not by
  counting — no race between counting and claiming.
- **GitLab.com Free cannot be pooled for concurrency.** The design supports
  GitLab for *correctness* (coordination primitive + ephemeral repos) but its
  scaling axis is GitHub-only. This is a forge constraint, not an
  implementation gap: the per-user rate limit has no API-provisionable lever on
  Free. GitLab scale, if needed, requires a paid tier or self-managed instance.
- A crashed run leaves at most one stale slot that self-heals via the staleness
  check within 15 minutes; during that window the pool has one fewer slot.
- The Playwright dependency for GitHub App installation means fully automated
  pool expansion requires a browser-capable environment. Org creation and
  inference provisioning are API-only and can run headless.
- Until the slot mechanism replaces the exclusive lock, per-org concurrency is
  1: the pool scales only by number of orgs, and each org serves one run at a
  time.
