package sessionconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSkills_SkillsDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills", "commit")
	mkdirAll(t, skillsDir)
	writeFile(t, filepath.Join(skillsDir, "SKILL.md"), `---
name: commit
description: Organize and create commits.
---

Run git commit logic here.`)

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "commit" {
		t.Errorf("name = %q, want commit", s.Name)
	}
	if s.Description != "Organize and create commits." {
		t.Errorf("description = %q", s.Description)
	}
	if s.Body != "Run git commit logic here." {
		t.Errorf("body = %q", s.Body)
	}
}

func TestDiscoverSkills_CommandsSubdir(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, ".claude", "commands", "ci")
	mkdirAll(t, cmdDir)
	writeFile(t, filepath.Join(cmdDir, "SKILL.md"), `---
name: ci
description: Run CI pipeline.
---

Run pnpm ci.`)

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "ci" {
		t.Errorf("name = %q, want ci", skills[0].Name)
	}
}

func TestDiscoverSkills_CommandsFlatFile(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, ".claude", "commands")
	mkdirAll(t, cmdDir)
	writeFile(t, filepath.Join(cmdDir, "release.md"), `---
description: Tag a new release.
---

Run git tag.`)

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "release" {
		t.Errorf("name = %q, want release (from filename)", s.Name)
	}
	if s.Description != "Tag a new release." {
		t.Errorf("description = %q", s.Description)
	}
}

func TestDiscoverSkills_DiscobotSkillsDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".discobot", "skills", "deploy")
	mkdirAll(t, skillsDir)
	writeFile(t, filepath.Join(skillsDir, "SKILL.md"), "---\nname: deploy\ndescription: Deploy via discobot.\n---\nDeploy.")

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "deploy" {
		t.Errorf("name = %q, want deploy", skills[0].Name)
	}
	if skills[0].Description != "Deploy via discobot." {
		t.Errorf("description = %q", skills[0].Description)
	}
}

func TestDiscoverSkills_ClaudeTakesPriorityOverDiscobot(t *testing.T) {
	root := t.TempDir()

	// Same skill in both .claude and .discobot — .claude wins.
	claudeDir := filepath.Join(root, ".claude", "skills", "deploy")
	mkdirAll(t, claudeDir)
	writeFile(t, filepath.Join(claudeDir, "SKILL.md"), "---\nname: deploy\ndescription: Claude version.\n---\nClaude deploy.")

	discobotDir := filepath.Join(root, ".discobot", "skills", "deploy")
	mkdirAll(t, discobotDir)
	writeFile(t, filepath.Join(discobotDir, "SKILL.md"), "---\nname: deploy\ndescription: Discobot version.\n---\nDiscobot deploy.")

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduped), got %d", len(skills))
	}
	if skills[0].Description != "Claude version." {
		t.Errorf("description = %q, want .claude to take priority", skills[0].Description)
	}
}

func TestDiscoverSkills_Deduplication(t *testing.T) {
	root := t.TempDir()

	// Create same skill name in both skills/ and commands/
	skillsDir := filepath.Join(root, ".claude", "skills", "deploy")
	mkdirAll(t, skillsDir)
	writeFile(t, filepath.Join(skillsDir, "SKILL.md"), "---\nname: deploy\ndescription: Deploy from skills.\n---\nDeploy content.")

	cmdDir := filepath.Join(root, ".claude", "commands", "deploy")
	mkdirAll(t, cmdDir)
	writeFile(t, filepath.Join(cmdDir, "SKILL.md"), "---\nname: deploy\ndescription: Deploy from commands.\n---\nCommand content.")

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduped), got %d", len(skills))
	}
	// skills/ takes priority over commands/
	if skills[0].Description != "Deploy from skills." {
		t.Errorf("description = %q, want skills/ version", skills[0].Description)
	}
}

func TestDiscoverSkills_MissingDirs(t *testing.T) {
	root := t.TempDir()
	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for empty dir, got %d", len(skills))
	}
}

func TestDiscoverSkills_SkipsNonDirsInSkillsDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills")
	mkdirAll(t, skillsDir)
	// A loose .md file in skills/ should be ignored (must be in subdirectory).
	writeFile(t, filepath.Join(skillsDir, "loose.md"), "---\nname: loose\n---\nShould be ignored.")

	skills, err := discoverSkillsWithHome(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (loose file in skills/ ignored), got %d", len(skills))
	}
}

func TestLookupSkill_FoundInSkillsDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "skills", "myskill")
	mkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: myskill\ndescription: My skill.\n---\nDo the thing.")

	skill, found, err := LookupSkill(root, "myskill")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected skill to be found")
	}
	if skill.Name != "myskill" {
		t.Errorf("name = %q", skill.Name)
	}
	if skill.Body != "Do the thing." {
		t.Errorf("body = %q", skill.Body)
	}
}

func TestLookupSkill_DoesNotSearchCommands(t *testing.T) {
	root := t.TempDir()
	// A command in commands/ should NOT be found by LookupSkill.
	dir := filepath.Join(root, ".claude", "commands", "release")
	mkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: release\n---\nTag it.")

	_, found, err := LookupSkill(root, "release")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("LookupSkill should not find commands/ entries")
	}
}

func TestLookupCommand_FoundInSubdir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "commands", "release")
	mkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: release\n---\nTag it.")

	cmd, found, err := LookupCommand(root, "release")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected command to be found")
	}
	if cmd.Body != "Tag it." {
		t.Errorf("body = %q", cmd.Body)
	}
}

func TestLookupCommand_FoundInFlatFile(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, ".claude", "commands")
	mkdirAll(t, cmdDir)
	writeFile(t, filepath.Join(cmdDir, "check.md"), "---\ndescription: Run checks.\n---\nRun checks.")

	cmd, found, err := LookupCommand(root, "check")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected command to be found")
	}
	if cmd.Name != "check" {
		t.Errorf("name = %q", cmd.Name)
	}
}

func TestLookupCommand_DoesNotSearchSkills(t *testing.T) {
	root := t.TempDir()
	// A skill in skills/ should NOT be found by LookupCommand.
	dir := filepath.Join(root, ".claude", "skills", "deploy")
	mkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, "SKILL.md"), "---\nname: deploy\n---\nDeploy it.")

	_, found, err := LookupCommand(root, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("LookupCommand should not find skills/ entries")
	}
}

func TestLookupSkill_NotFound(t *testing.T) {
	root := t.TempDir()
	_, found, err := LookupSkill(root, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestFormatSkillsReminder_Empty(t *testing.T) {
	got := FormatSkillsReminder(nil)
	if got != "" {
		t.Errorf("expected empty string for nil skills, got %q", got)
	}
}

func TestFormatSkillsReminder_WithSkills(t *testing.T) {
	skills := []SkillConfig{
		{Name: "commit", Description: "Create commits."},
		{Name: "release", Description: "Tag a release."},
		{Name: "nodesc"},
	}
	got := FormatSkillsReminder(skills)

	if !strings.Contains(got, "<system-reminder>") {
		t.Error("missing <system-reminder> tag")
	}
	if !strings.Contains(got, "commit: Create commits.") {
		t.Error("missing commit skill")
	}
	if !strings.Contains(got, "release: Tag a release.") {
		t.Error("missing release skill")
	}
	if !strings.Contains(got, "- nodesc\n") {
		t.Error("missing nodesc skill (no description)")
	}
	if !strings.Contains(got, "</system-reminder>") {
		t.Error("missing </system-reminder> tag")
	}
}

func TestParseSkill_NoFrontmatter(t *testing.T) {
	skill, err := parseSkill("myname", "Just the body content.")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "myname" {
		t.Errorf("name = %q, want myname", skill.Name)
	}
	if skill.Body != "Just the body content." {
		t.Errorf("body = %q", skill.Body)
	}
	if skill.Description != "" {
		t.Errorf("description should be empty, got %q", skill.Description)
	}
}

func TestParseSkill_FrontmatterOverridesName(t *testing.T) {
	content := "---\nname: override-name\ndescription: Desc here.\n---\nBody text."
	skill, err := parseSkill("default-name", content)
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "override-name" {
		t.Errorf("name = %q, want override-name", skill.Name)
	}
	if skill.Description != "Desc here." {
		t.Errorf("description = %q", skill.Description)
	}
	if skill.Body != "Body text." {
		t.Errorf("body = %q", skill.Body)
	}
}
