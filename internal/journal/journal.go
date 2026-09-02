// Package journal manages the append-only journal.xlsx workbook: creation
// with a header, atomic per-student saves, and lock detection so a locked
// file never crashes the app (Task.md checks 9-12).
package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ErrLocked — journal.xlsx is held by an office application; the caller may retry.
var ErrLocked = errors.New("journal.xlsx занят")

// ErrBlankSheet — the sheet has rows but every one of them is blank; refusing to
// guess a target row rather than risk writing over row 1.
var ErrBlankSheet = errors.New("лист содержит только пустые строки")

var headers = []string{"Last name", "First name", "Номер билета", "Дата и время"}

// Journal manages append-only writes to a single xlsx workbook.
type Journal struct {
	path string
}

// New builds a Journal over path. Injectable so tests use t.TempDir().
func New(path string) *Journal {
	return &Journal{path: path}
}

// Default builds a Journal over the standard journal.xlsx in the working directory.
func Default() *Journal {
	return New("journal.xlsx")
}

// lockMarkerPaths returns the marker filenames to probe for the given xlsx path,
// in the journal's own directory (D5):
//
//	filepath.Join(dir, "~$"+base)          — Microsoft Excel owner file
//	filepath.Join(dir, ".~lock."+base+"#") — LibreOffice / ONLYOFFICE style
func lockMarkerPaths(path string) []string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	return []string{
		filepath.Join(dir, "~$"+base),
		filepath.Join(dir, ".~lock."+base+"#"),
	}
}

// nextRow returns the 1-based row number to write into:
// (index of the last row holding a non-whitespace cell) + 2.
// Returns ErrBlankSheet if no row holds one. Callers must handle len(rows)==0
// before calling (that is the header-creation path, not an error).
func nextRow(rows [][]string) (int, error) {
	for i := len(rows) - 1; i >= 0; i-- {
		for _, cell := range rows[i] {
			if strings.TrimSpace(cell) != "" {
				return i + 2, nil
			}
		}
	}
	return 0, ErrBlankSheet
}

// Append adds exactly one row. Creates the file with a header if absent.
// Existing rows are never read-modified-written by value.
func (j *Journal) Append(last, first string, ticket int, when time.Time) error {
	for _, marker := range lockMarkerPaths(j.path) {
		if _, err := os.Stat(marker); err == nil {
			return ErrLocked
		}
	}

	var f *excelize.File
	var sheet string
	var row int

	if _, err := os.Stat(j.path); errors.Is(err, fs.ErrNotExist) {
		f = excelize.NewFile()
		sheet = "Sheet1"
		if err := writeHeader(f, sheet); err != nil {
			return fmt.Errorf("не удалось записать шапку: %w", err)
		}
		row = 2
	} else {
		f, err = excelize.OpenFile(j.path)
		if err != nil {
			return fmt.Errorf("не удалось открыть файл: %w", err)
		}
		sheet = f.GetSheetName(0)
		rows, err := f.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("не удалось прочитать строки: %w", err)
		}
		if len(rows) == 0 {
			if err := writeHeader(f, sheet); err != nil {
				return fmt.Errorf("не удалось записать шапку: %w", err)
			}
			row = 2
		} else {
			n, err := nextRow(rows)
			if err != nil {
				return err
			}
			row = n
		}
	}
	defer f.Close()

	cell := fmt.Sprintf("A%d", row)
	values := &[]any{last, first, ticket, when.Format("2006-01-02 15:04:05")}
	if err := f.SetSheetRow(sheet, cell, values); err != nil {
		return fmt.Errorf("не удалось записать строку: %w", err)
	}

	return saveAtomic(f, j.path)
}

// writeHeader writes the column headers into row 1 of a freshly-created (or
// empty) sheet, bold, with fixed column widths.
func writeHeader(f *excelize.File, sheet string) error {
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	styleID, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	if err := f.SetRowStyle(sheet, 1, 1, styleID); err != nil {
		return err
	}
	return f.SetColWidth(sheet, "A", "D", 18)
}

// saveAtomic writes f to a temp file, fsyncs it, then renames it over path —
// a kill -9 mid-save leaves either the old file or the new one, never a
// truncated one (N1, Task.md:32).
//
// Deviation from SPECS §5.7 step 6 (tmp = path + ".tmp"): excelize.SaveAs
// rejects any filename whose extension is not in its own supportedContentTypes
// map (file.go:76, checked against filepath.Ext at runtime) and returns
// ErrWorkbookFileFormat for anything else, ".tmp" included — confirmed by
// running the app against a real ~$journal.xlsx lock marker (AC-12a), where
// every retry failed with "unsupported workbook file format". The tmp name
// keeps the .xlsx extension instead, so SaveAs accepts it while the
// tmp+fsync+rename atomicity is unchanged.
func saveAtomic(f *excelize.File, path string) error {
	ext := filepath.Ext(path)
	tmp := strings.TrimSuffix(path, ext) + ".tmp" + ext
	if err := f.SaveAs(tmp); err != nil {
		os.Remove(tmp)
		if errors.Is(err, fs.ErrPermission) {
			return ErrLocked
		}
		return fmt.Errorf("не удалось сохранить временный файл: %w", err)
	}

	tf, err := os.OpenFile(tmp, os.O_RDWR, 0)
	if err != nil {
		os.Remove(tmp)
		if errors.Is(err, fs.ErrPermission) {
			return ErrLocked
		}
		return fmt.Errorf("не удалось открыть временный файл: %w", err)
	}
	if err := tf.Sync(); err != nil {
		tf.Close()
		os.Remove(tmp)
		return fmt.Errorf("не удалось синхронизировать временный файл: %w", err)
	}
	if err := tf.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("не удалось закрыть временный файл: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		if errors.Is(err, fs.ErrPermission) {
			return ErrLocked
		}
		return fmt.Errorf("не удалось переименовать временный файл: %w", err)
	}
	return nil
}
