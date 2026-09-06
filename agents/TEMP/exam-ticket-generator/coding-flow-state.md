# coding-flow state — exam-ticket-generator

Request: Implement the exam ticket generator console app per Task.md — student name
input, ticket number, journal.xlsx append, ESC to exit.

Size: MEDIUM (~4-8 files, one area). Safest/longest path selected per prerequisite 4.

## Environment (verified 2026-09-02)

- Go 1.26.2 darwin/arm64 — installed, GOPROXY reachable
- Node v24.14.1, Python 3.9.6 — also available
- dotnet, java — NOT installed
- Repo: single commit `d74d420`, branch `main`
- No Rosetta project docs (`docs/CONTEXT.md`, `docs/ARCHITECTURE.md`, `agents/*`) — greenfield.
  Suggest `init-workspace-flow` later; not blocking for a lab-sized task.

## Pre-existing artifacts

- `Task.md` — the assignment (authoritative source of requirements)
- `PLAN.md` — a detailed pre-existing Go implementation plan (excelize + x/term),
  untracked, authorship unknown. Authority NOT confirmed → HITL question raised.

## Phase ledger

| # | Phase | Applies | Status |
|---|---|---|---|
| 0 | prerequisites | ALL | done — skills loaded: load-project-context, hitl, orchestration |
| 1 | discovery | gaps exist | in_progress — env done; HITL questions open |
| 2 | design (architect, opus) | ALL | pending |
| 3 | user_review_design (HITL) | ALL | pending |
| 4 | tech_plan (architect, opus) | ALL | done — SPECS + PLAN written under `plans/exam-ticket-generator/` |
| 5 | review_plan (reviewer) | MEDIUM | done — verdict READY WITH FIXES; 2 blockers, 3 majors |
| 6 | user_review_plan (HITL) | ALL | **APPROVED** by user 2026-09-02: "Yes I reviewed the plan. Implement" |
| 7 | implementation (engineer) | ALL | done — 4 files, build+vet silent. One forced deviation (see below). |
| 8 | review_code (reviewer) | ALL | done — APPROVED WITH FIXES; 0 blockers, 0 majors |
| 9 | impl_validation (validator) | MEDIUM | done — pty harness built; 12/12 targets PASS, 0 defects |
| 10 | user_review_impl (HITL) | ALL | **APPROVED** by user 2026-09-02: "everything is good". Committed `b7309af`, pushed to `origin/main`. |
| 11 | tests (engineer) | ALL | done — 25 tests, 3 files, coverage main 25.6% / journal 62.3% / keys 51.1% |
| 12 | review_tests (reviewer) | MEDIUM | done — APPROVED WITH FIXES; 2 fixes applied and re-verified |
| 13 | final_validation (validator) | MEDIUM | skipped — build/vet/test already re-verified independently by orchestrator; no new behavior, tests-only change |

## Post-ship additions (2026-09-03)

- README.md rewritten, `.github/workflows/ci.yml` + `auto-pr.yml` added.
  Branch `chore/readme-and-ci` → PR #1, CI green on ubuntu+macos, merge pending user.
- Repo now has branch→auto-PR flow. Tests phase (below) should go out the same way:
  a branch, not a direct push to main.

## Open questions (Phase 1 HITL) — ALL ANSWERED 2026-09-02

## Decisions log

| # | Decision | Source | Effect |
|---|---|---|---|
| D1 | `PLAN.md` is APPROVED as written — implement it | user, 2026-09-02 | Go + excelize + `x/term`; 4-file layout; atomic tmp+rename save. Phases 2-3 satisfied. |
| D2 | Target OS is macOS only | user, 2026-09-02 | `~$journal.xlsx` marker = primary lock detector. `fs.ErrPermission` kept as cheap secondary check only. |
| D3 | ESC exits at BOTH name prompts | user, 2026-09-02 | **AMENDS PLAN.md §2 item 7**, which restricted ESC to `Last name:`. `PromptWithESC` now used for both fields. ESC at `First name:` discards the partial entry. |
| D4 | Ticket numbers may repeat — independent random 1..20 | user, 2026-09-02 | Matches PLAN.md §3 item 3. No uniqueness state. |

| D5 | Lock detection = marker files, BOTH names: `~$journal.xlsx` and `.~lock.journal.xlsx#` | user, 2026-09-02 | `lsof` approach explicitly rejected. ACCEPTED WITH KNOWN RISK — unverified that ONLYOFFICE writes either marker. AC-12 may not fire. Must not be overclaimed. |
| D6 | Rune-read fix APPROVED (architect defect #2) | user, 2026-09-02 | `ReadRune` instead of byte read. Cyrillic first letters echo correctly. Amends PLAN.md §2 item 5. |
| D7 | Arrow-key refinement REJECTED (architect defect #10) | user, 2026-09-02 | Revert to PLAN.md as approved: ANY `0x1b` exits. Cut the `Buffered()` discrimination, the Ctrl+D quit key, and AC-KEY. KISS wins. |

D3, D6 are scope amendments on top of D1. D7 restores D1. All settled at the Phase 6 gate.

## Phase 5 outcome — reviewer verdict: READY WITH FIXES

Orchestrator adjudication of reviewer findings:

| Finding | Reviewer said | Orchestrator ruling |
|---|---|---|
| B1 — S0 step needs a human to open Excel | blocker | SUPERSEDED. Microsoft Excel is not installed on this machine (only ONLYOFFICE). The whole `~$` premise collapses → user question Q1. |
| B2 — TTY checks unrunnable from a piped shell | blocker | ACCEPTED. ESC/raw-mode checks need the user at a real terminal. Plan must say so plainly. |
| M1 — arrow-key discrimination is new scope | cut it | ESCALATED to user (Q3) — user-visible, contradicts D1. |
| M2 — testability apparatus is speculative | trim it | REJECTED. Reviewer did not know coding-flow Phase 11 mandates tests. Injectable seams are required, keep them. |
| M3 — SPECS/PLAN duplicate each other | trim | ACCEPTED, doc hygiene. Architect to fix, no user input needed. |
| Architect defect #2 — byte read garbles Cyrillic echo | correct fix | ESCALATED to user (Q2) — user-visible, contradicts D1. |

## Blocking discovery — 2026-09-02

`/Applications/Microsoft Excel.app` does NOT exist. Only `ONLYOFFICE.app` is installed.
The `~$journal.xlsx` owner-file marker is Microsoft Excel behavior. Task.md check 12
("open journal.xlsx in Excel and try to add a student") therefore has no verifiable
path as designed. D2 rests on a premise that is false on this machine.

## Implementation deviation — APPROVED, forced by runtime reality

SPECS §5.7 step 6 specified the temp file as `journal.xlsx.tmp`. `excelize.File.SaveAs`
rejects any extension outside `{.xlsx .xlsm .xltx .xltm .xlam}` and returns
`ErrWorkbookFileFormat`, so EVERY `Append` would have failed unconditionally.
Source: `excelize/v2@v2.11.0/file.go:76` + `templates.go:820-826`.

Fix: temp file is `journal.tmp.xlsx`. Atomicity via `os.Rename` is unaffected — same
directory, single syscall. Verified independently by the reviewer with a standalone
probe against the pinned dependency: `SaveAs(.tmp)` errors, `SaveAs(.tmp.xlsx)` succeeds.

Found by execution, not by reading. Neither the architect nor the reviewer caught it
statically — this is why the plan mandated running the code.

## AC-12b — RESOLVED, check 12 CONFIRMED WORKING (2026-09-02)

The open risk R1 is closed on real evidence, not a synthetic marker. The user opened
`journal.xlsx` in ONLYOFFICE; a `.~lock.journal.xlsx#` file appeared in the directory,
containing:

    ,acopusciu,C13082,02.09.2026 10:19,/Users/acopusciu/.local/share/onlyoffice;

ONLYOFFICE writes exactly the marker name D5 specified. Task.md check 12 therefore
triggers for real on this machine. `AC-12b` = SATISFIED. Nothing about D5 needs changing.

## Non-finding — `README.md`

Reviewer flagged `README.md` as modified against the ownership rule. It was ALREADY
`M` in the git snapshot at session start, before any agent ran. Not caused by this
work. No action.

## Known concerns to resolve in specs

- PLAN.md §2 item 6 constructs `bufio.NewReader(os.Stdin)` per call. A per-call
  buffered reader can swallow bytes between calls. Needs a shared reader or
  unbuffered line assembly. Architect must resolve.
- D1 approval covers intent, not technical correctness. Architect must still flag
  genuine defects in PLAN.md; any material change escalates to the user.
