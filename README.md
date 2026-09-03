<div align="center">

# Exam Ticket Generator

**A console app that draws exam ticket numbers and keeps a tamper-safe Excel journal.**

[![CI](https://github.com/ArtemGCorp/PAD-Lab1/actions/workflows/ci.yml/badge.svg)](https://github.com/ArtemGCorp/PAD-Lab1/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-macOS-000000?logo=apple&logoColor=white)](#requirements)

PAD course · Laboratory work 1

</div>

---

## What it does

Ask for a student's name, draw them a ticket, write it down. Repeat until someone
presses <kbd>ESC</kbd>. Every record lands in `journal.xlsx` **the instant it is
entered** — not when the app closes — so pulling the plug mid-session costs you
nothing.

```console
$ ./pad-lab1
Для выхода нажмите ESC.
Last name: Иванов
First name: Пётр
Билет № 7
Last name: Петрова
First name: Анна
Билет № 15
Last name: ▮          ← ESC here exits
```

The resulting `journal.xlsx`:

| Last name | First name | Номер билета | Дата и время |
|:----------|:-----------|-------------:|:-------------|
| Иванов    | Пётр       |            7 | 2026-09-02 13:14:39 |
| Петрова   | Анна       |           15 | 2026-09-02 13:18:58 |

---

## Requirements

| | |
|---|---|
| **Go** | 1.26 or newer |
| **OS** | macOS (raw terminal mode is not gated behind build tags, but only macOS is tested) |
| **Terminal** | A real TTY — <kbd>ESC</kbd> cannot be captured through a pipe |

## Running it

```bash
git clone git@github.com:ArtemGCorp/PAD-Lab1.git
cd PAD-Lab1
go run .
```

Or build once and keep the binary:

```bash
go build -o pad-lab1 .
./pad-lab1
```

> [!TIP]
> `journal.xlsx` is created in **whatever directory you run from**. To keep the repo
> clean while testing, run from somewhere else:
> ```bash
> mkdir -p /tmp/lab1 && cd /tmp/lab1
> go run /path/to/PAD-Lab1
> ```

### Controls

| Key | Effect |
|---|---|
| <kbd>Enter</kbd> | Confirm the current field |
| <kbd>ESC</kbd> | Exit — works at **both** the last-name and first-name prompts |
| <kbd>Ctrl</kbd>+<kbd>C</kbd> | Exit (handled manually; raw mode disables signals) |

> [!NOTE]
> Arrow keys also exit. Terminals send them as escape sequences beginning with the
> same `0x1b` byte as <kbd>ESC</kbd>, and this app deliberately treats every `0x1b`
> as "quit" rather than parsing sequences. A conscious simplicity trade-off.

---

## How it works

```
main.go ──────────────── input loop, validation, retry-on-locked
   │
   ├── internal/keys ──── raw-mode first keystroke, ESC capture, echo
   │
   └── internal/journal ─ Excel append, atomic save, lock detection
```

| File | Lines | Responsibility |
|---|---:|---|
| [`main.go`](main.go) | 90 | Prompt loop, blank-input rejection, ticket draw, locked-file retry |
| [`internal/keys/keys.go`](internal/keys/keys.go) | 136 | Puts the terminal in raw mode for exactly one keystroke, classifies it, restores |
| [`internal/journal/journal.go`](internal/journal/journal.go) | 190 | Reads the sheet, appends a row, saves atomically, detects office locks |

### Design decisions worth knowing

<details>
<summary><b>Saves survive <code>kill -9</code></b></summary>

<br>

Writing straight into `journal.xlsx` would leave a truncated file if the process
died mid-write. Instead every save goes:

```
SaveAs(journal.tmp.xlsx) → fsync → rename(journal.tmp.xlsx, journal.xlsx)
```

`rename(2)` is atomic within a filesystem, so a reader sees either the old file or
the new one — never a half-written one. Every failure branch removes the temp file.

*Residual gap:* the containing directory is not fsynced, so a hard power loss (as
opposed to a process kill) could still lose the most recent entry. Deliberate — the
assignment tests `kill -9`, which is fully covered.

</details>

<details>
<summary><b>Why the temp file is <code>journal.tmp.xlsx</code>, not <code>journal.xlsx.tmp</code></b></summary>

<br>

The obvious name doesn't work. `excelize.File.SaveAs` validates the file extension
against a fixed set — `.xlsx`, `.xlsm`, `.xltx`, `.xltm`, `.xlam` — and returns
`ErrWorkbookFileFormat` for anything else:

```go
// excelize/v2@v2.11.0/file.go:76
if _, ok := supportedContentTypes[strings.ToLower(filepath.Ext(f.Path))]; !ok {
    return ErrWorkbookFileFormat
}
```

A `.tmp` suffix means **every single save fails**. Keeping `.xlsx` at the end sidesteps
it, and `rename` doesn't care that the two names have different stems.

</details>

<details>
<summary><b>The first keystroke is read as a rune, not a byte</b></summary>

<br>

Raw mode disables echo, so the app must print the first character itself. Reading a
single *byte* breaks immediately on Cyrillic: `И` is `0xD0 0x98` in UTF-8, and echoing
`0xD0` alone paints a replacement glyph. `ReadRune` consumes the whole codepoint, so
the screen matches what was typed. The stored value was always correct — this is
purely about what you see while typing.

</details>

<details>
<summary><b>Detecting that the journal is open elsewhere</b></summary>

<br>

Office suites drop a sibling lock file next to the document. Both known names are
checked before writing:

| Marker | Written by |
|---|---|
| `~$journal.xlsx` | Microsoft Excel |
| `.~lock.journal.xlsx#` | ONLYOFFICE, LibreOffice |

When one is found the app prints a message and **retries the same student**, keeping
the ticket number that was already shown on screen, so no record is silently lost.

This matters because macOS gives no other signal: POSIX `rename` over a file another
process holds open *succeeds*, so there is no error to catch. The marker file is the
only reliable tell.

</details>

<details>
<summary><b>The terminal is always restored</b></summary>

<br>

Raw mode is entered for exactly one keystroke, inside one function, with
`defer term.Restore` — which runs during panic unwinding too. `os.Exit` is banned
anywhere in the `keys` package, since it would skip the deferred restore and leave
your shell without echo. The only unrecoverable case is `SIGKILL` landing inside that
one-keystroke window.

</details>

---

## Verification

Every requirement in the assignment was exercised against a real pseudo-terminal, with
results read back from the workbook via `openpyxl` rather than trusted from console
output.

| # | Requirement | Status |
|:-:|---|:-:|
| 9 | Three students, then ESC → header + 3 rows | ✅ |
| 10 | Re-run, two more students → 5 rows, first three untouched | ✅ |
| 11 | `kill -9` after an entry → record survives, file opens cleanly | ✅ |
| 12 | Journal open in an editor → clear message, no crash | ✅ |
| 13 | Empty or whitespace-only input → re-prompts, writes nothing | ✅ |

Check 12 was confirmed against a live ONLYOFFICE session, not just a synthetic marker.

---

## Development

```bash
go build ./...     # compile
go vet ./...       # static analysis
go test ./...      # unit tests
gofmt -l .         # formatting — must print nothing
```

### Contributing workflow

`main` is protected by convention: push a branch and let automation open the PR.

```bash
git switch -c feature/my-change
# ...edit...
git commit -am "Describe the change"
git push -u origin feature/my-change
```

Pushing any branch other than `main` triggers [`auto-pr.yml`](.github/workflows/auto-pr.yml),
which opens a pull request into `main`, assigns it to you, and requests your review —
so GitHub emails you. [`ci.yml`](.github/workflows/ci.yml) builds, vets, formats and
tests on both Linux and macOS. Merge once it's green.

```
push branch → PR opened automatically → email → CI runs → you merge
```

> [!IMPORTANT]
> One-time repo setup for the automation:
> **Settings → Actions → General → Workflow permissions** →
> select *Read and write permissions* and tick
> *Allow GitHub Actions to create and approve pull requests*.

---

## Project layout

```
.
├── main.go                       entry point and input loop
├── internal/
│   ├── journal/journal.go        Excel persistence
│   └── keys/keys.go              raw-mode keyboard handling
├── .github/workflows/
│   ├── ci.yml                    build · vet · gofmt · test
│   └── auto-pr.yml               branch push → pull request
├── Task.md                       the assignment
├── PLAN.md                       original implementation plan
└── plans/                        technical specs and execution plan
```

## Built with

- [`github.com/xuri/excelize/v2`](https://github.com/xuri/excelize) — reading and writing `.xlsx`
- [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) — cross-platform raw terminal mode
