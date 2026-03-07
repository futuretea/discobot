package cli

// terminal.go — raw-mode line reader with persistent command history.
//
// readLine puts stdin into raw mode for each line so that arrow keys and
// other control sequences can be intercepted before the kernel line discipline
// consumes them. History is saved to disk after every new entry.

import (
	"bytes"
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

type pastedChunk struct {
	end     int
	rawLen  int
	dispLen int
}

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
		buf      []byte
		histIdx  int    // index into hist.entries; len(hist.entries) = "current input"
		histSave string // user's in-progress text, saved when navigating away
		pastes   []pastedChunk
	)
	if hist != nil {
		histIdx = len(hist.entries)
	}

	for {
		lead := make([]byte, 1)
		if _, err := os.Stdin.Read(lead); err != nil {
			return string(buf), err
		}

		pastes = normalizePastedChunks(pastes, len(buf))

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
			if len(buf) == 0 {
				break
			}
			if len(pastes) > 0 {
				last := pastes[len(pastes)-1]
				if len(buf) == last.end {
					start := last.end - last.rawLen
					if start < 0 {
						start = 0
					}
					buf = buf[:start]
					pastes = pastes[:len(pastes)-1]
					eraseVisual(last.dispLen)
					break
				}
			}
			_, n := utf8.DecodeLastRune(buf)
			if n < 1 {
				n = 1
			}
			buf = buf[:len(buf)-n]
			eraseVisual(1)

		case 0x1b: // ESC — start of a control sequence
			r, code, params := readEscapeSequence()
			if r != 0 {
				// Printable character encoded in an escape (unusual but safe).
				buf = append(buf, string(r)...)
				fmt.Fprintf(os.Stderr, "%c", r)
				break
			}
			if code == '~' && params == "200" {
				pasted, err := readBracketedPaste()
				if err != nil {
					return string(buf), err
				}
				buf = append(buf, pasted...)
				summary := pastedSummary(pasted)
				pastes = append(pastes, pastedChunk{end: len(buf), rawLen: len(pasted), dispLen: len(summary)})
				fmt.Fprint(os.Stderr, summary)
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
				buf = []byte(hist.entries[histIdx])
				pastes = nil

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
				buf = []byte(next)
				pastes = nil
			}

		default:
			if lead[0] < 0x20 {
				break // ignore other control characters
			}
			_, raw, err := decodeRune(lead[0])
			if err != nil {
				break
			}
			buf = append(buf, raw...)
			fmt.Fprint(os.Stderr, string(raw))
		}
	}
}

// eraseVisual moves the cursor back n columns, overwrites with spaces, and
// moves back again — effectively deleting n visual characters from the display.
func eraseVisual(n int) {
	if n <= 0 {
		return
	}
	fmt.Fprint(os.Stderr, strings.Repeat("\b", n)+strings.Repeat(" ", n)+strings.Repeat("\b", n))
}

// setLineContent replaces the visible line with prompt + newContent using ANSI
// erase-line so there are no leftover characters from the previous content.
// Leading newlines are stripped from prompt because \r moves to the start of
// the current line — a newline would push the replacement onto the next line.
func setLineContent(prompt, newContent string) {
	fmt.Fprintf(os.Stderr, "\r\033[2K%s%s", strings.TrimLeft(prompt, "\n"), newContent)
}

// readEscapeSequence reads a CSI escape sequence from stdin (the ESC byte has
// already been consumed). It returns either a printable rune (r != 0), or a
// command final-byte plus parameter string (for example, code='~', params="200"
// for bracketed-paste start).
func readEscapeSequence() (r rune, code byte, params string) {
	// Peek at next byte to detect CSI "[".
	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil || b[0] != '[' {
		return 0, 0, ""
	}

	// Read parameter bytes (digits and semicolons) followed by a final byte.
	var p []byte
	for {
		if _, err := os.Stdin.Read(b); err != nil {
			return 0, 0, ""
		}
		c := b[0]
		if (c >= '0' && c <= '9') || c == ';' {
			p = append(p, c)
			continue
		}
		return 0, c, string(p)
	}
}

func normalizePastedChunks(chunks []pastedChunk, bufLen int) []pastedChunk {
	for len(chunks) > 0 {
		last := chunks[len(chunks)-1]
		start := last.end - last.rawLen
		if start >= 0 && last.end <= bufLen {
			break
		}
		chunks = chunks[:len(chunks)-1]
	}
	return chunks
}

func readBracketedPaste() ([]byte, error) {
	term := []byte{0x1b, '[', '2', '0', '1', '~'}
	var out []byte
	one := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(one); err != nil {
			return nil, err
		}
		out = append(out, one[0])
		if len(out) >= len(term) && bytes.Equal(out[len(out)-len(term):], term) {
			out = out[:len(out)-len(term)]
			return out, nil
		}
	}
}

func pastedSummary(data []byte) string {
	return fmt.Sprintf("[pasted %d lines/%d bytes]", pastedLineCount(data), len(data))
}

func pastedLineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'}) + 1
	if data[len(data)-1] == '\n' {
		lines--
	}
	if lines < 1 {
		lines = 1
	}
	return lines
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
