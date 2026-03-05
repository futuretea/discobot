package cli

// terminal.go — raw-mode line reader with persistent command history.
//
// readLine puts stdin into raw mode for each line so that arrow keys and
// other control sequences can be intercepted before the kernel line discipline
// consumes them. History is saved to disk after every new entry.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// errInterrupt is returned by readLine when the user presses Ctrl+C.
var errInterrupt = errors.New("interrupted")

const maxHistoryEntries = 500

// cmdHistory stores per-session command history and persists it across restarts.
type cmdHistory struct {
	entries []string
	path    string
}

// loadCmdHistory reads history from path. Missing file is not an error.
func loadCmdHistory(path string) *cmdHistory {
	h := &cmdHistory{path: path}
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line != "" {
				h.entries = append(h.entries, line)
			}
		}
		if len(h.entries) > maxHistoryEntries {
			h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
		}
	}
	return h
}

// push appends line to the history and saves it to disk.
// Adjacent duplicates are skipped.
func (h *cmdHistory) push(line string) {
	if line == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == line {
		return
	}
	h.entries = append(h.entries, line)
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[1:]
	}
	_ = h.save()
}

func (h *cmdHistory) save() error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(h.path, []byte(strings.Join(h.entries, "\n")+"\n"), 0o600)
}

// readLine prints prompt to stderr, then reads one line from stdin in raw
// terminal mode. If hist is non-nil, the up/down arrow keys navigate history.
//
// Returns:
//   - (line, nil)         — normal input
//   - ("", errInterrupt)  — Ctrl+C
//   - ("", io.EOF)        — Ctrl+D on empty buffer, or stdin closed
func readLine(prompt string, hist *cmdHistory) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	// Non-interactive fallback (piped stdin).
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return readLineSimple()
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return readLineSimple()
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck

	var (
		buf      []rune
		histIdx  int    // index into hist.entries; len(hist.entries) = "current input"
		histSave string // user's in-progress text, saved when navigating away
	)
	if hist != nil {
		histIdx = len(hist.entries)
	}

	for {
		lead := make([]byte, 1)
		if _, err := os.Stdin.Read(lead); err != nil {
			return string(buf), err
		}

		switch lead[0] {
		case 0x03: // Ctrl+C
			fmt.Fprint(os.Stderr, "^C\r\n")
			return "", errInterrupt

		case 0x04: // Ctrl+D — EOF only when buffer is empty
			if len(buf) == 0 {
				fmt.Fprint(os.Stderr, "\r\n")
				return "", io.EOF
			}

		case 0x0d, 0x0a: // Enter (CR or LF)
			fmt.Fprint(os.Stderr, "\r\n")
			return string(buf), nil

		case 0x7f, 0x08: // Backspace / Delete
			if len(buf) > 0 {
				last := buf[len(buf)-1]
				buf = buf[:len(buf)-1]
				w := utf8.RuneLen(last)
				if w < 1 {
					w = 1
				}
				fmt.Fprint(os.Stderr, "\b"+strings.Repeat(" ", w)+strings.Repeat("\b", w))
			}

		case 0x1b: // ESC — start of a control sequence
			r, code := readEscapeSequence()
			if r != 0 {
				// Printable character encoded in an escape (unusual but safe).
				buf = append(buf, r)
				fmt.Fprintf(os.Stderr, "%c", r)
				break
			}
			switch code {
			case 'A': // Up arrow
				if hist == nil || histIdx <= 0 {
					break
				}
				if histIdx == len(hist.entries) {
					histSave = string(buf)
				}
				histIdx--
				setLineContent(prompt, hist.entries[histIdx])
				buf = []rune(hist.entries[histIdx])

			case 'B': // Down arrow
				if hist == nil || histIdx >= len(hist.entries) {
					break
				}
				histIdx++
				next := histSave
				if histIdx < len(hist.entries) {
					next = hist.entries[histIdx]
				}
				setLineContent(prompt, next)
				buf = []rune(next)
			}

		default:
			if lead[0] < 0x20 {
				break // ignore other control characters
			}
			r, raw, err := decodeRune(lead[0])
			if err != nil {
				break
			}
			buf = append(buf, r)
			fmt.Fprint(os.Stderr, string(raw))
		}
	}
}

// setLineContent replaces the visible line with prompt + newContent using ANSI
// erase-line so there are no leftover characters from the previous content.
// Leading newlines are stripped from prompt because \r moves to the start of
// the current line — a newline would push the replacement onto the next line.
func setLineContent(prompt, newContent string) {
	fmt.Fprintf(os.Stderr, "\r\033[2K%s%s", strings.TrimLeft(prompt, "\n"), newContent)
}

// readEscapeSequence reads a CSI escape sequence from stdin (the ESC byte has
// already been consumed). It returns either a printable rune (r != 0) if the
// sequence resolves to one, or a single-byte command code such as 'A'/'B' for
// arrow keys. Unknown or malformed sequences return (0, 0).
func readEscapeSequence() (r rune, code byte) {
	// Peek at next byte to detect CSI "[".
	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil || b[0] != '[' {
		return 0, 0
	}

	// Read parameter bytes (digits and semicolons) followed by a final byte.
	var params []byte
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			return 0, 0
		}
		c := b[0]
		if (c >= '0' && c <= '9') || c == ';' {
			params = append(params, c)
		} else {
			// c is the final byte of the sequence.
			_ = params // could be used to handle e.g. \e[1~ (Home)
			return 0, c
		}
	}
}

// decodeRune reads a full UTF-8 rune from stdin given its already-read lead byte.
func decodeRune(lead byte) (rune, []byte, error) {
	var n int
	switch {
	case lead < 0x80:
		n = 1
	case lead < 0xE0:
		n = 2
	case lead < 0xF0:
		n = 3
	default:
		n = 4
	}
	raw := make([]byte, n)
	raw[0] = lead
	if n > 1 {
		if _, err := io.ReadFull(os.Stdin, raw[1:]); err != nil {
			return utf8.RuneError, raw[:1], err
		}
	}
	rv, _ := utf8.DecodeRune(raw)
	return rv, raw, nil
}

// readLineSimple reads a line from os.Stdin byte-by-byte without raw mode.
// Used as fallback when stdin is not a terminal or MakeRaw fails.
func readLineSimple() (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			if len(buf) > 0 {
				return strings.TrimRight(string(buf), "\r\n"), nil
			}
			return "", err
		}
		if b[0] == '\n' {
			return strings.TrimRight(string(buf), "\r"), nil
		}
		buf = append(buf, b[0])
	}
}
