package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillConfig represents a discovered skill (user-invocable prompt template).
type SkillConfig struct {
	// Name is the skill's slash-command name (e.g., "commit").
	Name string

	// Description describes what the skill does. Claude uses this to decide
	// when to auto-invoke the skill and for autocomplete.
	Description string

	// Body is the markdown prompt content (after frontmatter is stripped).
	Body string

	// Kind is "skill" for entries from skills/ directories and "command" for
	// entries from commands/ directories.
	Kind string
}

// discoverSkills loads skill configs from the project's .claude/skills and
// .claude/commands directories, plus ~/.claude/skills and ~/.claude/commands.
// The equivalent .discobot/ directories are also checked as an alternative
// naming style.
// Priority: project skills (.claude then .discobot) → user skills →
// project commands → user commands.
// Later entries with a duplicate name are ignored.
func discoverSkills(projectRoot string) ([]SkillConfig, error) {
	home, _ := os.UserHomeDir()
	return discoverSkillsWithHome(projectRoot, home)
}

func discoverSkillsWithHome(projectRoot, home string) ([]SkillConfig, error) {
	var skills []SkillConfig
	seen := make(map[string]bool)

	add := func(s SkillConfig) {
		if !seen[s.Name] {
			seen[s.Name] = true
			skills = append(skills, s)
		}
	}

	addFrom := func(list []SkillConfig, err error) error {
		if err != nil {
			return err
		}
		for _, s := range list {
			add(s)
		}
		return nil
	}

	// 1. Project skills: .claude/skills/*/SKILL.md then .discobot/skills/*/SKILL.md
	for _, dir := range []string{".claude", ".discobot"} {
		if err := addFrom(loadSkillsDir(filepath.Join(projectRoot, dir, "skills"))); err != nil {
			return nil, err
		}
	}

	// 2. User skills: ~/.claude/skills/*/SKILL.md then ~/.discobot/skills/*/SKILL.md
	if home != "" {
		for _, dir := range []string{".claude", ".discobot"} {
			if err := addFrom(loadSkillsDir(filepath.Join(home, dir, "skills"))); err != nil {
				return nil, err
			}
		}
	}

	// 3. Project commands: .claude/commands/ then .discobot/commands/ (both formats).
	for _, dir := range []string{".claude", ".discobot"} {
		if err := addFrom(loadCommandsDir(filepath.Join(projectRoot, dir, "commands"))); err != nil {
			return nil, err
		}
	}

	// 4. User commands: ~/.claude/commands/ then ~/.discobot/commands/ (both formats).
	if home != "" {
		for _, dir := range []string{".claude", ".discobot"} {
			if err := addFrom(loadCommandsDir(filepath.Join(home, dir, "commands"))); err != nil {
				return nil, err
			}
		}
	}

	return skills, nil
}

// LookupSkill searches for a skill by name in skills/ directories only.
// It does NOT search commands/ — use LookupCommand for legacy commands.
// Both .claude and .discobot directory styles are checked.
// Returns (zero, false, nil) if the skill is not found.
func LookupSkill(projectRoot, skillName string) (SkillConfig, bool, error) {
	var paths []string
	for _, dir := range []string{".claude", ".discobot"} {
		paths = append(paths, filepath.Join(projectRoot, dir, "skills", skillName, "SKILL.md"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{".claude", ".discobot"} {
			paths = append(paths, filepath.Join(home, dir, "skills", skillName, "SKILL.md"))
		}
	}
	skill, ok, err := lookupFirst(skillName, paths)
	if ok {
		skill.Kind = "skill"
	}
	return skill, ok, err
}

// LookupCommand searches for a legacy command by name in commands/ directories.
// Commands are expanded programmatically when a user message starts with /name,
// unlike skills which are invoked via the Skill tool by the LLM.
// Both .claude and .discobot directory styles are checked.
// Returns (zero, false, nil) if the command is not found.
func LookupCommand(projectRoot, cmdName string) (SkillConfig, bool, error) {
	var paths []string
	for _, dir := range []string{".claude", ".discobot"} {
		paths = append(paths,
			filepath.Join(projectRoot, dir, "commands", cmdName, "SKILL.md"),
			filepath.Join(projectRoot, dir, "commands", cmdName+".md"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, dir := range []string{".claude", ".discobot"} {
			paths = append(paths,
				filepath.Join(home, dir, "commands", cmdName, "SKILL.md"),
				filepath.Join(home, dir, "commands", cmdName+".md"),
			)
		}
	}
	cmd, ok, err := lookupFirst(cmdName, paths)
	if ok {
		cmd.Kind = "command"
	}
	return cmd, ok, err
}

// lookupFirst returns the config for the first path that exists and parses successfully.
func lookupFirst(defaultName string, paths []string) (SkillConfig, bool, error) {
	for _, p := range paths {
		content, err := readFileIfExists(p)
		if err != nil {
			return SkillConfig{}, false, fmt.Errorf("read skill file %s: %w", p, err)
		}
		if content == "" {
			continue
		}
		skill, err := parseSkill(defaultName, content)
		if err != nil {
			return SkillConfig{}, false, fmt.Errorf("parse skill %s: %w", p, err)
		}
		return skill, true, nil
	}
	return SkillConfig{}, false, nil
}

// loadSkillsDir loads all skills from a directory where each subdirectory
// contains a SKILL.md file.
func loadSkillsDir(dir string) ([]SkillConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills dir %s: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var skills []SkillConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
		content, err := readFileIfExists(skillFile)
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", skillFile, err)
		}
		if content == "" {
			continue
		}
		skill, err := parseSkill(e.Name(), content)
		if err != nil {
			return nil, fmt.Errorf("parse skill %s: %w", e.Name(), err)
		}
		skill.Kind = "skill"
		skills = append(skills, skill)
	}
	return skills, nil
}

// loadCommandsDir loads skills from a .claude/commands directory.
// Supports subdirectory format (name/SKILL.md) and flat format (name.md).
func loadCommandsDir(dir string) ([]SkillConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read commands dir %s: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var skills []SkillConfig
	for _, e := range entries {
		if e.IsDir() {
			// Subdirectory format: commands/name/SKILL.md
			skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
			content, err := readFileIfExists(skillFile)
			if err != nil {
				return nil, fmt.Errorf("read command %s: %w", skillFile, err)
			}
			if content == "" {
				continue
			}
			skill, err := parseSkill(e.Name(), content)
			if err != nil {
				return nil, fmt.Errorf("parse command %s: %w", e.Name(), err)
			}
			skill.Kind = "command"
			skills = append(skills, skill)
		} else if strings.HasSuffix(e.Name(), ".md") {
			// Flat format: commands/name.md
			p := filepath.Join(dir, e.Name())
			content, err := readFileIfExists(p)
			if err != nil {
				return nil, fmt.Errorf("read command %s: %w", p, err)
			}
			if content == "" {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			skill, err := parseSkill(name, content)
			if err != nil {
				return nil, fmt.Errorf("parse command %s: %w", name, err)
			}
			skill.Kind = "command"
			skills = append(skills, skill)
		}
	}
	return skills, nil
}

// Expand substitutes $ARGUMENTS (and the $0 shorthand) in the skill body with
// args. If args is non-empty and neither placeholder appears in the body, the
// args are appended as "ARGUMENTS: <args>".
func (s SkillConfig) Expand(args string) string {
	body := s.Body
	if args == "" {
		body = strings.ReplaceAll(body, "$ARGUMENTS", "")
		body = strings.ReplaceAll(body, "$0", "")
		return strings.TrimSpace(body)
	}

	replaced := false
	if strings.Contains(body, "$ARGUMENTS") {
		body = strings.ReplaceAll(body, "$ARGUMENTS", args)
		replaced = true
	}
	if strings.Contains(body, "$0") {
		body = strings.ReplaceAll(body, "$0", args)
		replaced = true
	}
	if !replaced {
		body = body + "\n\nARGUMENTS: " + args
	}
	return body
}

// parseSkill parses a skill markdown file (with optional YAML frontmatter)
// into a SkillConfig. defaultName is used when the frontmatter omits a name.
func parseSkill(defaultName, content string) (SkillConfig, error) {
	fm, body, err := parseFrontmatter(content)
	if err != nil {
		return SkillConfig{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	skill := SkillConfig{
		Name: defaultName,
		Body: strings.TrimSpace(body),
	}

	if fm != nil {
		if name, ok := fm["name"].(string); ok && name != "" {
			skill.Name = name
		}
		if desc, ok := fm["description"].(string); ok {
			skill.Description = desc
		}
	}

	return skill, nil
}

// FormatSkillsReminder formats the list of available skills as a
// <system-reminder> block. Returns empty string if skills is empty.
func FormatSkillsReminder(skills []SkillConfig) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	b.WriteString("The following skills are available for use with the Skill tool:\n\n")

	for _, s := range skills {
		fmt.Fprintf(&b, "- %s", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, ": %s", s.Description)
		}
		b.WriteString("\n")
	}

	b.WriteString("\nWhen users reference a \"slash command\" or \"/<something>\" (e.g., \"/commit\", \"/review-pr\"), they are referring to a skill. Use the Skill tool to invoke it.")
	b.WriteString("\n</system-reminder>")
	return b.String()
}
