package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pad-lab1/internal/journal"
	"pad-lab1/internal/keys"

	"github.com/xuri/excelize/v2"
)

// --- formatTicket (pure) ----------------------------------------------------

func TestFormatTicket(t *testing.T) {
	if got := formatTicket(7); got != "Билет № 7" {
		t.Fatalf("formatTicket(7) = %q, want %q", got, "Билет № 7")
	}
}

// TestFormatTicket_UsesU2116Glyph guards against a lookalike substitution
// (ASCII "No", "#", etc.) that would satisfy a naive string-equality check
// against a literal typed the same way by mistake, but would fail the human
// grader reading Task.md's exact graded line (SPECS §5.9).
func TestFormatTicket_UsesU2116Glyph(t *testing.T) {
	got := formatTicket(7)

	// Scan by codepoint rather than comparing against another string literal
	// typed the same way — that would not catch a lookalike (ASCII "No",
	// "#", U+2007 etc.) if the same mistake were made in both places. Find
	// the rune from the Letterlike Symbols block and check its exact value.
	var found rune
	ok := false
	for _, r := range got {
		if r >= 0x2100 && r <= 0x214F {
			found = r
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("formatTicket(7) = %q contains no Letterlike Symbols block rune", got)
	}
	if found != 0x2116 {
		t.Fatalf("formatTicket(7) numero sign rune = U+%04X, want U+2116 (№)", found)
	}
}

// --- saveEntry: retry preserves the same ticket and timestamp --------------
//
// journal.Append is stateless and has no notion of "retry" (each call takes
// ticket/when as plain parameters) — journal_test.go's TestAppend_* prove
// only that Append is callable again once a lock marker is gone, using a
// fresh time.Now() on each call. The property SPECS §5.8 actually requires —
// that a retry after ErrLocked reuses the EXACT ticket and timestamp already
// shown on screen, never redrawing or re-stamping — lives entirely in
// saveEntry's retry loop (main.go) and can only be proven here.

// removeMarkerOnRead wraps a reader and deletes markerPath the first time
// Read is called, simulating "operator removes the lock file, then presses
// Enter" — the read that unblocks saveEntry's ReadLine() is the same event
// that must observe the marker gone, mirroring the real control flow in
// main.go's saveEntry (ReadLine() is called, then the loop retries Append).
type removeMarkerOnRead struct {
	r          io.Reader
	markerPath string
	removed    bool
}

func (r *removeMarkerOnRead) Read(p []byte) (int, error) {
	if !r.removed {
		if err := os.Remove(r.markerPath); err != nil {
			return 0, err
		}
		r.removed = true
	}
	return r.r.Read(p)
}

func TestSaveEntry_RetryAfterLockPreservesTicketAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.xlsx")
	j := journal.New(path)

	markerPath := filepath.Join(dir, "~$journal.xlsx")
	if err := os.WriteFile(markerPath, []byte("locked"), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}

	// The exact values already "shown on screen" before the retry loop
	// starts — saveEntry must reuse these on the successful second Append,
	// never drawing a new ticket or re-stamping the time.
	const wantTicket = 13
	wantWhen := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)

	reader := &removeMarkerOnRead{
		r:          strings.NewReader("\n"),
		markerPath: markerPath,
	}
	p := keys.NewPrompter(reader, io.Discard, -1)

	ok := saveEntry(p, j, "Ivanov", "Ivan", wantTicket, wantWhen)
	if !ok {
		t.Fatalf("saveEntry() = false, want true (should succeed after marker removed)")
	}
	if !reader.removed {
		t.Fatalf("test setup error: marker was never removed via ReadLine's read")
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker still present after saveEntry: err = %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1 entry)", len(rows))
	}

	row := rows[1]
	wantRow := []string{"Ivanov", "Ivan", "13", wantWhen.Format("2006-01-02 15:04:05")}
	for i, w := range wantRow {
		if row[i] != w {
			t.Errorf("cell[%d] = %q, want %q (must equal the value passed to saveEntry, not a fresh draw)", i, row[i], w)
		}
	}
}
