package keys

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// --- classify (pure) -------------------------------------------------------
//
// Table covers every byte named in SPECS §4.1 as actually implemented after
// D7: ESC and Ctrl+C quit, CR/LF are Enter, DEL/Backspace are ignored (first
// key has nothing to erase), everything else is a plain char. No arrow-key
// escape-sequence discrimination and no Ctrl+D special-casing exist (D7) —
// asserted below by checking Ctrl+D classifies as an ordinary char.

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want keyClass
	}{
		{"ESC quits", 0x1b, classQuit},
		{"Ctrl+C quits", 0x03, classQuit},
		{"CR is Enter", 0x0d, classEnter},
		{"LF is Enter", 0x0a, classEnter},
		{"DEL is ignored", 0x7f, classIgnore},
		{"Backspace is ignored", 0x08, classIgnore},
		{"ordinary letter is a char", 'a', classChar},
		{"ordinary digit is a char", '5', classChar},
		{"Cyrillic letter is a char", 'И', classChar},
		{"space is a char", ' ', classChar},
		{"Ctrl+D has no special case (post-D7)", 0x04, classChar},
		{"Ctrl+Z has no special case", 0x1a, classChar},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.r); got != tc.want {
				t.Errorf("classify(%q) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

// --- Prompter / NewPrompter: non-TTY (fd < 0) path -------------------------
//
// Passing fd = -1 makes term.IsTerminal(fd) irrelevant — the fd < 0 guard in
// Prompt short-circuits straight to the cooked-mode line read, exactly the
// case SPECS §9 calls out as reachable without a real pty.

func TestPrompt_NonTTY_ReadsCookedLine(t *testing.T) {
	in := strings.NewReader("Ivanov\n")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	line, ok, err := p.Prompt("Last name: ")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !ok {
		t.Fatalf("Prompt() ok = false, want true")
	}
	if line != "Ivanov\n" {
		t.Fatalf("Prompt() line = %q, want %q (raw, untrimmed)", line, "Ivanov\n")
	}
	if out.String() != "Last name: " {
		t.Fatalf("label written to out = %q, want %q", out.String(), "Last name: ")
	}
}

func TestPrompt_NonTTY_EOFWithNoInput(t *testing.T) {
	in := strings.NewReader("")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	line, ok, err := p.Prompt("First name: ")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if ok {
		t.Fatalf("Prompt() ok = true on empty EOF input, want false")
	}
	if line != "" {
		t.Fatalf("Prompt() line = %q, want empty on EOF", line)
	}
}

func TestPrompt_NonTTY_EOFWithoutTrailingNewline(t *testing.T) {
	// No trailing \n before EOF: readCookedLine's prefix=="" && rest==""
	// branch does not apply since rest holds the partial line, so it must
	// still be returned with ok=true.
	in := strings.NewReader("Petrov")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	line, ok, err := p.Prompt("Last name: ")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !ok {
		t.Fatalf("Prompt() ok = false, want true for a non-empty partial line")
	}
	if line != "Petrov" {
		t.Fatalf("Prompt() line = %q, want %q", line, "Petrov")
	}
}

func TestPrompt_NonTTY_EmptyLine(t *testing.T) {
	// A blank line (just "\n") is itself a valid cooked-mode read; trimming
	// blank input is main.promptNonEmpty's job, not this package's.
	in := strings.NewReader("\n")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	line, ok, err := p.Prompt("Last name: ")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !ok {
		t.Fatalf("Prompt() ok = false, want true")
	}
	if line != "\n" {
		t.Fatalf("Prompt() line = %q, want %q", line, "\n")
	}
}

func TestPrompt_NonTTY_WhitespaceLine(t *testing.T) {
	// Whitespace-only content must round-trip untouched — Prompt does not
	// trim; that is main's job.
	in := strings.NewReader("   \n")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	line, ok, err := p.Prompt("Last name: ")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !ok {
		t.Fatalf("Prompt() ok = false, want true")
	}
	if line != "   \n" {
		t.Fatalf("Prompt() line = %q, want %q", line, "   \n")
	}
}

func TestPrompt_NonTTY_MultipleSequentialPrompts(t *testing.T) {
	in := strings.NewReader("Ivanov\nIvan\n")
	var out bytes.Buffer
	p := NewPrompter(in, &out, -1)

	last, ok, err := p.Prompt("Last name: ")
	if err != nil || !ok {
		t.Fatalf("first Prompt() = (%q, %v, %v)", last, ok, err)
	}
	if last != "Ivanov\n" {
		t.Fatalf("first Prompt() line = %q, want %q", last, "Ivanov\n")
	}

	first, ok, err := p.Prompt("First name: ")
	if err != nil || !ok {
		t.Fatalf("second Prompt() = (%q, %v, %v)", first, ok, err)
	}
	if first != "Ivan\n" {
		t.Fatalf("second Prompt() line = %q, want %q", first, "Ivan\n")
	}

	wantOut := "Last name: First name: "
	if out.String() != wantOut {
		t.Fatalf("accumulated out = %q, want %q", out.String(), wantOut)
	}
}

// --- Prompter.ReadLine (used for the "press Enter to retry" flow) ---------

func TestReadLine(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		wantOk bool
	}{
		{"newline present", "\n", true},
		{"content then newline", "anything\n", true},
		{"EOF with no newline", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPrompter(strings.NewReader(tc.in), &bytes.Buffer{}, -1)
			ok, err := p.ReadLine()
			if err != nil {
				t.Fatalf("ReadLine() error = %v", err)
			}
			if ok != tc.wantOk {
				t.Fatalf("ReadLine() ok = %v, want %v", ok, tc.wantOk)
			}
		})
	}
}

// --- non-terminal fd (fd >= 0 but not a real terminal) ---------------------

// errReader lets us exercise Prompt's non-EOF error path without a real fd.
type errReader struct{ err error }

func (r errReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestPrompt_NonTTY_ReadError(t *testing.T) {
	sentinel := io.ErrClosedPipe
	p := NewPrompter(errReader{sentinel}, &bytes.Buffer{}, -1)

	_, ok, err := p.Prompt("Last name: ")
	if ok {
		t.Fatalf("Prompt() ok = true, want false on read error")
	}
	if err != sentinel {
		t.Fatalf("Prompt() error = %v, want %v", err, sentinel)
	}
}

// NewStdinPrompter is a thin constructor wrapper (os.Stdin/os.Stdout/its fd);
// nothing to assert beyond it not panicking, since exercising its raw-mode
// path requires a real TTY (see report's UNTESTABLE section).
func TestNewStdinPrompter_DoesNotPanic(t *testing.T) {
	p := NewStdinPrompter()
	if p == nil {
		t.Fatal("NewStdinPrompter() returned nil")
	}
}
