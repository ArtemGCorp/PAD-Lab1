// Package keys implements the single-keystroke ESC/Enter capture used to
// prompt for student names: one raw-mode key read (to catch ESC before it
// would otherwise be swallowed by a line-buffered read), then a normal
// cooked-mode line read for the rest of the input.
package keys

import (
	"bufio"
	"io"
	"os"

	"golang.org/x/term"
)

// Prompter owns the ONLY reader over its input stream.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
	fd  int
}

// NewPrompter builds a Prompter. fd < 0, or a non-terminal fd, disables raw
// mode: Prompt degrades to a plain line read (ESC is then unreachable, which
// is correct for pipes and tests).
func NewPrompter(in io.Reader, out io.Writer, fd int) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out, fd: fd}
}

// NewStdinPrompter builds the process's single stdin/stdout Prompter.
func NewStdinPrompter() *Prompter {
	return NewPrompter(os.Stdin, os.Stdout, int(os.Stdin.Fd()))
}

type keyClass int

const (
	classChar keyClass = iota
	classQuit
	classEnter
	classIgnore
)

// classify maps a raw first keystroke to its meaning. Retained as a named
// function rather than inlined into the read loop: it is the only
// branch-dense logic in this package and it is unreachable from a test once
// inlined (the loop around it needs a TTY). N5 makes this seam mandatory.
func classify(r rune) keyClass {
	switch r {
	case 0x1b: // ESC — any 0x1b exits, no escape-sequence discrimination (D7)
		return classQuit
	case 0x03: // Ctrl+C — raw mode disables SIGINT; handled explicitly
		return classQuit
	case 0x0d, 0x0a: // CR (raw-mode Enter) and LF (piped/odd terminals)
		return classEnter
	case 0x7f, 0x08: // DEL, BS — nothing to erase yet; re-read the first key
		return classIgnore
	default:
		return classChar
	}
}

// Prompt writes label, reads one line. Returns the RAW (untrimmed) line.
// ok=false means the caller must terminate the program (ESC, Ctrl+C, or EOF).
func (p *Prompter) Prompt(label string) (string, bool, error) {
	io.WriteString(p.out, label)

	if p.fd < 0 || !term.IsTerminal(p.fd) {
		return p.readCookedLine("")
	}

	old, err := term.MakeRaw(p.fd)
	if err != nil {
		return p.readCookedLine("")
	}
	defer term.Restore(p.fd, old)

	for {
		r, _, err := p.in.ReadRune()
		if err != nil {
			term.Restore(p.fd, old)
			if err == io.EOF {
				return "", false, nil
			}
			return "", false, err
		}

		switch classify(r) {
		case classQuit:
			term.Restore(p.fd, old)
			io.WriteString(p.out, "\n")
			return "", false, nil
		case classEnter:
			term.Restore(p.fd, old)
			io.WriteString(p.out, "\n")
			return "", true, nil
		case classIgnore:
			continue
		case classChar:
			// Restore the terminal now, before any further I/O, so the
			// manual echo below and the kernel's own echo of the rest of
			// the line agree on cursor state (SPECS §5.1 step 7-8).
			term.Restore(p.fd, old)
			io.WriteString(p.out, string(r))
			return p.readCookedLine(string(r))
		}
	}
}

// readCookedLine finishes reading a line in cooked mode, prefixing any
// already-consumed rune.
func (p *Prompter) readCookedLine(prefix string) (string, bool, error) {
	rest, err := p.in.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			if prefix == "" && rest == "" {
				return "", false, nil
			}
			return prefix + rest, true, nil
		}
		return "", false, err
	}
	return prefix + rest, true, nil
}

// ReadLine reads a line with no raw phase — used for "press Enter to retry".
// ok=false means EOF.
func (p *Prompter) ReadLine() (bool, error) {
	_, err := p.in.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
