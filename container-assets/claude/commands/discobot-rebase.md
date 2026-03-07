---
name: discobot-rebase
description: Rebase session changes onto the target workspace commit
argument-hint: <commit-id>
disable-model-invocation: true
---

Rebase this session's git history onto target commit $ARGUMENTS.

1. **Inspect current state:** Run `git status` and `git log --oneline -5` to confirm current HEAD and working tree state.

2. **Prepare changes for rebase as needed:**
   - If there are uncommitted local changes, choose the safest path to make rebase possible.
   - You may commit changes when appropriate, but you are **not required** to fully finalize all staged content for workspace transfer.
   - Keep changes staged if that is the best path for resolving rebase conflicts.

3. **Rebase onto target commit:**
   - Ensure the target commit exists locally: `git rev-parse --verify "$ARGUMENTS^{commit}"`.
   - If it does not exist, fetch refs from origin and verify again.
   - Run `git rebase --autostash $ARGUMENTS` to rebase current history onto the target commit while preserving uncommitted work.

4. **Resolve conflicts when needed:**
   - If rebase conflicts occur, surface them clearly to the user.
   - Use `git status` to list conflicts.
   - After resolving conflicts, continue with `git rebase --continue`.
   - If the user asks to stop, use `git rebase --abort`.

5. **Verify:**
   - Confirm the sandbox history is rebased onto `$ARGUMENTS`.
   - Ensure git state is coherent before finishing.
