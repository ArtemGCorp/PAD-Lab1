// Command pad-lab1 is the exam ticket generator console app: it asks for a
// student's last and first name, draws a ticket number 1..20, and appends
// the entry to journal.xlsx. ESC exits at either prompt.
package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"pad-lab1/internal/journal"
	"pad-lab1/internal/keys"
)

// formatTicket renders the ticket announcement, e.g. "Билет № 7".
func formatTicket(n int) string {
	return fmt.Sprintf("Билет № %d", n)
}

func main() {
	p := keys.NewStdinPrompter()
	j := journal.Default()

	fmt.Println("Для выхода нажмите ESC.")

	for {
		last, ok := promptNonEmpty(p, "Last name: ")
		if !ok {
			return
		}

		first, ok := promptNonEmpty(p, "First name: ")
		if !ok {
			return
		}

		ticket := rand.IntN(20) + 1
		when := time.Now()
		fmt.Println(formatTicket(ticket))

		if !saveEntry(p, j, last, first, ticket, when) {
			return
		}
	}
}

// promptNonEmpty repeats label until a non-blank line is entered, or the
// operator quits (ESC/Ctrl+C/EOF).
func promptNonEmpty(p *keys.Prompter, label string) (string, bool) {
	for {
		text, ok, err := p.Prompt(label)
		if !ok {
			return "", false
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка чтения ввода: %v\n", err)
			return "", false
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		return trimmed, true
	}
}

// saveEntry appends the student's entry, retrying on ErrLocked with the same
// ticket and timestamp until it succeeds or the operator quits. Returns
// false only when the program should exit.
func saveEntry(p *keys.Prompter, j *journal.Journal, last, first string, ticket int, when time.Time) bool {
	for {
		err := j.Append(last, first, ticket, when)
		if err == nil {
			return true
		}
		if errors.Is(err, journal.ErrLocked) {
			fmt.Println("Файл journal.xlsx открыт в Excel. Закройте его и нажмите Enter для повтора.")
			ok, readErr := p.ReadLine()
			if readErr != nil || !ok {
				return false
			}
			continue
		}
		fmt.Fprintf(os.Stderr, "Не удалось сохранить запись: %v\n", err)
		return true
	}
}
