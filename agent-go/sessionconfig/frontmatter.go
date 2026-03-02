package sessionconfig

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// parseFrontmatter extracts YAML frontmatter from a markdown document.
// Frontmatter is delimited by "---" lines at the start of the file.
// Returns the parsed frontmatter as a map, the remaining body, and any error.
// If no frontmatter is present, returns nil map and the original content.
func parseFrontmatter(content string) (map[string]any, string, error) {
	trimmed := strings.TrimLeft(content, "\n\r")
	if !strings.HasPrefix(trimmed, "---") {
		return nil, content, nil
	}

	// Find the closing "---" delimiter.
	rest := trimmed[3:]
	// Skip the rest of the opening delimiter line.
	if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
		rest = rest[idx+1:]
	} else {
		// Only "---" with nothing after it.
		return nil, content, nil
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		// No closing delimiter — treat as no frontmatter.
		return nil, content, nil
	}

	yamlContent := rest[:closeIdx]
	body := rest[closeIdx+4:] // skip "\n---"
	// Skip trailing newline after closing delimiter.
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, content, err
	}

	return fm, body, nil
}
