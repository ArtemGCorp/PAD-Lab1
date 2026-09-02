# Execution Plan — exam-ticket-generator

Single session. Contracts, behavior and acceptance criteria live in `exam-ticket-generator-SPECS.md`. This file owns **order, ordering traps, and verification**, and points into SPECS by section rather than restating it. Where a trap below names an identifier, it is a pointer to the contract, not a substitute for reading it.

Read first: `Task.md` · `exam-ticket-generator-SPECS.md` · `PLAN.md` (superseded where SPECS §11 says so) · `agents/TEMP/exam-ticket-generator/coding-flow-state.md` (D1–D7).

Rules:
- SPECS §11 overrides `PLAN.md` line-for-line — do not "restore" the original, and do not re-add anything in §11's *rejected* table.
- D2 macOS only: no build tags, no `runtime.GOOS` branches.
- D7: any `0x1b` exits. No escape-sequence handling of any kind.
- D5: marker-file lock detection only. No `lsof`, no process inspection.
- N5 is mandatory — the injectable seams and pure helpers in SPECS §9 are contract, not polish. A tests session follows this one.
- Root of all work: `/Users/acopusciu/универ/4curs/PAD/Lab1`.

Ownership: this session owns every file it creates. Nothing under `agents/`, `Task.md`, `PLAN.md`, `README.md`, or `plans/` is edited.

---

## S1 — Module and dependencies

Do: `go mod init pad-lab1`; `go get github.com/xuri/excelize/v2`; `go get golang.org/x/term`.

Verify: `go.mod` names both; `go build ./...` exits 0.

## S2 — `internal/journal/journal.go`

Do: implement SPECS §4.2, §5.7, §6, §7. Pure helpers (`nextRow`, `lockMarkerPaths`) first, then `Append`.

Ordering traps — the things a competent agent gets wrong by default here:
1. `SetSheetRow` takes a **pointer** to the slice (SPECS §10 A2).
2. Create path and open path resolve the sheet name differently (SPECS §5.7 steps 2 vs 3). Getting one hardcoded constant for both is the default mistake.
3. The lock pre-check runs **before** the file is opened, or a lock is reported only after work is wasted.
4. `when` and `ticket` are parameters. `Append` must not call `time.Now()` or draw a number — the retry loop in SPECS §5.8 depends on that.
5. Failure branches in SPECS §5.7 step 7 are easy to leave partial; every one of them removes `tmp`.

Verify: `go build ./...` and `go vet ./...` silent.

## S3 — `internal/keys/keys.go`

Do: implement SPECS §4.1, §5.1–§5.6.

Ordering traps:
1. Exactly one `bufio.NewReader` in the entire codebase (SPECS §5.2). Reading `os.Stdin` directly anywhere in this package reintroduces the defect.
2. The step order in SPECS §5.1 is load-bearing: restore precedes the manual echo, and both precede the line read. Any other order breaks the display.
3. `term.MakeRaw` in exactly one function; `os.Exit` in none (SPECS §5.4).
4. `ReadRune`, not `ReadByte` (D6).

Verify: `go build ./...`, `go vet ./...` silent. `grep -rn 'os.Exit\|bufio.NewReader\|ReadByte' internal/ main.go` — no `os.Exit` in `internal/keys`, no `ReadByte`, exactly one `bufio.NewReader`.

## S4 — `main.go`

Do: implement SPECS §4.3, §5.6, §5.8, §5.9 and the state machine in SPECS §3.

Ordering traps:
1. One `keys.NewStdinPrompter()` for the whole process — constructing a second one anywhere defeats S3 trap 1.
2. `when := time.Now()` and the ticket draw happen **once per student**, before the first `Append`, and are reused across every retry (SPECS §5.8).
3. The five literals in SPECS §5.9 are copied character-for-character. `№` is U+2116, not `No`. The graded strings live there.
4. `ErrLocked` retries; every other error does not (SPECS §5.8 last paragraph).

Verify: `go build ./...` and `go vet ./...` produce **no output** and exit 0.

## S5 — Acceptance

SPECS §8 holds the criteria and marks each row `HUMAN` or `AGENT`. The split is not a convenience: `Prompter` falls back to cooked mode whenever stdin is not a terminal (SPECS §5.1 step 2), so **piped input never reaches the raw-mode path**. An agent feeding stdin from a pipe or heredoc verifies nothing about ESC, Enter-as-CR, or first-key echo, and must not report those rows as passed.

### S5a — Agent-runnable now, no TTY needed

Do, and record output:
- `AC-BLD` — `go build ./...` and `go vet ./...`, both empty output, exit 0.
- `AC-12a` — synthetic lock. In a scratch dir: `touch '~$journal.xlsx'`, drive a student entry, confirm the SPECS §5.9 message and no panic; `rm` the marker, press Enter, confirm the row lands with the same ticket. Repeat with `.~lock.journal.xlsx#`. Stdin may be piped here — this row does not touch the raw path.

### S5b — Human operator, real terminal required

Hand the operator this list. Each row is `HUMAN` in SPECS §8; the agent supplies the commands and then inspects the resulting file.

1. `AC-9` — empty scratch dir, run, 3 students, ESC at `Last name:`.
2. `AC-10` — run again on that file, 2 students, ESC.
3. `AC-11` — run, 1 student, wait for the next prompt, `kill -9 <pid>`.
4. `AC-13` — bare Enter, then `"   "` + Enter, at both fields.
5. `AC-D3` — valid surname, then ESC at `First name:`.
6. `AC-ECHO` — Backspace as the very first key; then `Иванов` — check the on-screen echo and the stored cell.
7. After **every** run: type `echo test`. Echo must be on. If it is not, the terminal was left raw — that is a defect, not an environment quirk.

Agent-side follow-up after S5b, no TTY needed: `AC-9f`, `AC-10f` (needs a copy of `journal.xlsx` taken between AC-9 and AC-10), `AC-11f`, `AC-13f`.

### S5c — Real-world lock observation (`AC-12b`, non-blocking)

Not a gate. Implementation and every other AC proceed regardless of the outcome.

Do: ask the operator to open `journal.xlsx` in **ONLYOFFICE** (Microsoft Excel is not installed on this machine), then run `ls -a` in that directory and report every sibling file that appeared. If `~$journal.xlsx` or `.~lock.journal.xlsx#` is among them, have them enter a student and confirm the locked message. If neither appears, record that verbatim and mark `AC-12b` **not satisfied**.

Do **not** report `Task.md` check 12 as passed on the strength of `AC-12a` alone — see SPECS §10 R1. `AC-12a` proves the code path; only `AC-12b` proves the trigger.

## S6 — Report

Do: report results per AC, every deviation actually taken, the S5c observation verbatim, and the honest status of check 12. Do not open a tests phase — separate later session, no `_test.go` files here.

---

## Findings (fill in during execution)

- S5c: sibling files observed while `journal.xlsx` was open in ONLYOFFICE: _TBD_
- `AC-12b` satisfied? _TBD_
- Any SPECS deviation required at implementation time: _TBD_

## Done when

- Four files exist: `go.mod`, `main.go`, `internal/journal/journal.go`, `internal/keys/keys.go`. No others.
- `go build ./...` and `go vet ./...` both produce empty output and exit 0.
- Every SPECS §8 row is recorded as passed, or as blocked with a stated reason. `AC-12b` may legitimately be recorded as not satisfied.
- Terminal echo verified working after each S5b run.

## Checklist

Contracts are in SPECS; this list checks that they were honored, not what they say.

- [ ] `go.mod` = `pad-lab1`, both deps present
- [ ] `internal/journal` matches SPECS §4.2 signatures exactly, incl. `ErrBlankSheet`
- [ ] `Append` follows SPECS §5.7 steps 1–7 in order
- [ ] S2 traps 1–5 each checked in the written code
- [ ] `nextRow` returns `ErrBlankSheet`, never `1`
- [ ] Both D5 marker names probed
- [ ] `internal/keys` matches SPECS §4.1 signatures exactly
- [ ] `classify` covers exactly the SPECS §4.1 table — no arrow branch, no `0x04` (D7)
- [ ] S3 traps 1–4 each checked in the written code
- [ ] SPECS §5.4 restoration table holds for every listed path
- [ ] `main.go` matches SPECS §4.3; S4 traps 1–4 each checked
- [ ] ESC exits at **both** prompts (D3)
- [ ] All five SPECS §5.9 literals byte-identical
- [ ] Injectable seams and pure helpers per SPECS §9 all present and exported-or-package-visible as specified
- [ ] `go build ./...` silent
- [ ] `go vet ./...` silent
- [ ] S5a run, output recorded
- [ ] S5b handed to the operator; results recorded per row
- [ ] S5c observation recorded verbatim, `AC-12b` status stated honestly
- [ ] Nothing in SPECS §11's *rejected* table reintroduced
- [ ] No `_test.go` files written (separate session)
- [ ] `Task.md`, `PLAN.md`, `README.md`, `agents/**` untouched — confirm with `git status`
