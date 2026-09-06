package journal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// --- nextRow (pure) ---------------------------------------------------

func TestNextRow(t *testing.T) {
	cases := []struct {
		name    string
		rows    [][]string
		wantRow int
		wantErr error
	}{
		{
			name:    "header only, no data rows",
			rows:    [][]string{{"Last name", "First name", "Номер билета", "Дата и время"}},
			wantRow: 2,
		},
		{
			name: "single data row after header",
			rows: [][]string{
				{"Last name", "First name", "Номер билета", "Дата и время"},
				{"Ivanov", "Ivan", "5", "2026-01-01 10:00:00"},
			},
			wantRow: 3,
		},
		{
			name: "trailing blank rows are skipped",
			rows: [][]string{
				{"Last name", "First name", "Номер билета", "Дата и время"},
				{"Ivanov", "Ivan", "5", "2026-01-01 10:00:00"},
				{},
				{"", "", "", ""},
			},
			wantRow: 3,
		},
		{
			name: "trailing whitespace-only rows are skipped",
			rows: [][]string{
				{"Last name", "First name", "Номер билета", "Дата и время"},
				{"Ivanov", "Ivan", "5", "2026-01-01 10:00:00"},
				{"  ", "\t", "", " "},
			},
			wantRow: 3,
		},
		{
			name: "data in the middle followed by blanks still finds it",
			rows: [][]string{
				{"Last name", "First name", "Номер билета", "Дата и время"},
				{"Ivanov", "Ivan", "5", "2026-01-01 10:00:00"},
				{"Petrov", "Petr", "12", "2026-01-01 11:00:00"},
				{},
				{},
			},
			wantRow: 4,
		},
		{
			name:    "empty rows slice",
			rows:    [][]string{},
			wantErr: ErrBlankSheet,
		},
		{
			name:    "all rows blank",
			rows:    [][]string{{}, {"", ""}, {"  ", "\t"}},
			wantErr: ErrBlankSheet,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := nextRow(tc.rows)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("nextRow() err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("nextRow() unexpected err = %v", err)
			}
			if got != tc.wantRow {
				t.Fatalf("nextRow() = %d, want %d", got, tc.wantRow)
			}
		})
	}
}

// --- lockMarkerPaths (pure) --------------------------------------------

func TestLockMarkerPaths(t *testing.T) {
	dir := filepath.Join("some", "dir")
	path := filepath.Join(dir, "journal.xlsx")

	got := lockMarkerPaths(path)
	want := []string{
		filepath.Join(dir, "~$journal.xlsx"),
		filepath.Join(dir, ".~lock.journal.xlsx#"),
	}

	if len(got) != len(want) {
		t.Fatalf("lockMarkerPaths() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lockMarkerPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLockMarkerPaths_RootDir(t *testing.T) {
	// A bare filename (no directory component) must still resolve markers
	// into "." rather than an empty/garbage directory.
	got := lockMarkerPaths("journal.xlsx")
	want := []string{
		filepath.Join(".", "~$journal.xlsx"),
		filepath.Join(".", ".~lock.journal.xlsx#"),
	}
	if len(got) != len(want) {
		t.Fatalf("lockMarkerPaths() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lockMarkerPaths()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- Append: header creation --------------------------------------------

func TestAppend_CreatesFileWithHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.xlsx")
	j := New(path)

	when := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	if err := j.Append("Ivanov", "Ivan", 7, when); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (header + 1 entry)", len(rows))
	}

	wantHeader := []string{"Last name", "First name", "Номер билета", "Дата и время"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], h)
		}
	}

	styleID, err := f.GetCellStyle(sheet, "A1")
	if err != nil {
		t.Fatalf("GetCellStyle() error = %v", err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatalf("GetStyle() error = %v", err)
	}
	if style.Font == nil || !style.Font.Bold {
		t.Errorf("header style Font.Bold = %v, want true", style.Font)
	}

	for _, col := range []string{"A", "B", "C", "D"} {
		width, err := f.GetColWidth(sheet, col)
		if err != nil {
			t.Fatalf("GetColWidth(%q) error = %v", col, err)
		}
		if width != 18 {
			t.Errorf("column %s width = %v, want 18", col, width)
		}
	}
}

// --- Append: value round-trip -------------------------------------------

func TestAppend_ValuesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.xlsx")
	j := New(path)

	when := time.Date(2026, 3, 4, 9, 8, 7, 0, time.UTC)
	if err := j.Append("Petrov", "Petr", 12, when); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	row := rows[1]
	want := []string{"Petrov", "Petr", "12", "2026-03-04 09:08:07"}
	for i, w := range want {
		if row[i] != w {
			t.Errorf("cell[%d] = %q, want %q", i, row[i], w)
		}
	}
}

// --- Append: existing file, sequential appends --------------------------

func TestAppend_ToExistingFile_PreservesPriorRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.xlsx")
	j := New(path)

	when1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	when2 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)

	if err := j.Append("Ivanov", "Ivan", 1, when1); err != nil {
		t.Fatalf("first Append() error = %v", err)
	}
	if err := j.Append("Petrov", "Petr", 2, when2); err != nil {
		t.Fatalf("second Append() error = %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("GetRows() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (header + 2 entries)", len(rows))
	}

	// Row 1 (index 0) untouched header, row 2 (index 1) untouched first
	// entry, row 3 (index 2) the new entry.
	if rows[1][0] != "Ivanov" || rows[1][2] != "1" {
		t.Errorf("first entry mutated: got %v", rows[1])
	}
	if rows[2][0] != "Petrov" || rows[2][2] != "2" {
		t.Errorf("second entry wrong: got %v", rows[2])
	}
}

func TestAppend_MultipleSequential_IncreasingRowOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.xlsx")
	j := New(path)

	names := []string{"Ivanov", "Petrov", "Sidorov", "Kuznetsov"}
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	for i, name := range names {
		when := base.Add(time.Duration(i) * time.Minute)
		if err := j.Append(name, "X", i+1, when); err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
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
	if len(rows) != len(names)+1 {
		t.Fatalf("got %d rows, want %d", len(rows), len(names)+1)
	}
	for i, name := range names {
		if rows[i+1][0] != name {
			t.Errorf("row %d = %q, want %q", i+1, rows[i+1][0], name)
		}
	}
}

// --- Append: locking ------------------------------------------------------

func TestAppend_ErrLocked(t *testing.T) {
	markerNames := []string{"~$journal.xlsx", ".~lock.journal.xlsx#"}

	for _, marker := range markerNames {
		t.Run(marker, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "journal.xlsx")
			j := New(path)

			markerPath := filepath.Join(dir, marker)
			if err := os.WriteFile(markerPath, []byte("locked"), 0o644); err != nil {
				t.Fatalf("WriteFile(marker) error = %v", err)
			}

			err := j.Append("Ivanov", "Ivan", 1, time.Now())
			if !errors.Is(err, ErrLocked) {
				t.Fatalf("Append() error = %v, want ErrLocked", err)
			}
		})
	}
}

func TestAppend_NotLockedWithoutMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.xlsx")
	j := New(path)

	err := j.Append("Ivanov", "Ivan", 1, time.Now())
	if errors.Is(err, ErrLocked) {
		t.Fatalf("Append() returned ErrLocked with no marker present")
	}
	if err != nil {
		t.Fatalf("Append() unexpected error = %v", err)
	}
}

// TestAppend_SucceedsOnceMarkerRemoved proves only that a fresh Append call
// succeeds once the lock marker is gone. It intentionally draws a new
// time.Now() for the second call, because Append is stateless per call and
// has no concept of "retry" — that is deliberately NOT what this test
// checks. It does NOT prove that a retry preserves the ticket/timestamp
// already shown on screen (SPECS §5.8); that property lives in
// main.saveEntry and is covered by
// TestSaveEntry_RetryAfterLockPreservesTicketAndTimestamp in main_test.go.
func TestAppend_SucceedsOnceMarkerRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.xlsx")
	j := New(path)

	markerPath := filepath.Join(dir, "~$journal.xlsx")
	if err := os.WriteFile(markerPath, []byte("locked"), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}

	if err := j.Append("Ivanov", "Ivan", 1, time.Now()); !errors.Is(err, ErrLocked) {
		t.Fatalf("Append() error = %v, want ErrLocked", err)
	}

	if err := os.Remove(markerPath); err != nil {
		t.Fatalf("Remove(marker) error = %v", err)
	}

	if err := j.Append("Ivanov", "Ivan", 1, time.Now()); err != nil {
		t.Fatalf("Append() after lock cleared, error = %v", err)
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
		t.Fatalf("got %d rows, want 2 (header + 1 successful entry)", len(rows))
	}
}

// --- Append: no leftover tmp file ------------------------------------------

func TestAppend_NoLeftoverTmpFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "journal.xlsx")
	j := New(path)

	if err := j.Append("Ivanov", "Ivan", 1, time.Now()); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if e.Name() != "journal.xlsx" {
			t.Errorf("unexpected leftover file in journal dir: %q", e.Name())
		}
	}
}
