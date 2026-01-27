# Header Component Test Scenarios

This document enumerates all the test scenarios for the Header component breadcrumb navigation.

## Test Scenarios

### Scenario 1: No workspaces exist
**Setup:**
- `workspaces = []`
- `selectedWorkspaceId = null`
- `selectedSession = null`

**Expected UI:**
- ✅ "Octobot" logo and text visible
- ✅ "Add Workspace" button visible (not dropdown)
- ✅ No session dropdown visible
- ✅ No "New Session" button visible (workspaces.length === 0)

**Breadcrumb structure:** `Octobot / [Add Workspace]`

---

### Scenario 2: Workspaces exist, none selected
**Setup:**
- `workspaces = [workspace1, workspace2]`
- `selectedWorkspaceId = null`
- `selectedSession = null`

**Expected UI:**
- ✅ Workspace dropdown shows "Select Workspace"
- ✅ No session dropdown visible
- ✅ "New Session" button visible (after final separator)

**Breadcrumb structure:** `Octobot / [Select Workspace] / [New Session]`

---

### Scenario 3: Workspace selected, no sessions exist
**Setup:**
- `workspaces = [workspace1]`
- `selectedWorkspaceId = "ws-1"`
- `workspaceSessions = []`
- `selectedSession = null`

**Expected UI:**
- ✅ Workspace dropdown shows "project1" (workspace name)
- ✅ No session dropdown visible (0 sessions)
- ✅ "New Session" button visible

**Breadcrumb structure:** `Octobot / [project1] / [New Session]`

---

### Scenario 4: Workspace selected, sessions exist, none selected
**Setup:**
- `workspaces = [workspace1]`
- `selectedWorkspaceId = "ws-1"`
- `workspaceSessions = [session1, session2]`
- `selectedSession = null`

**Expected UI:**
- ✅ Workspace dropdown shows "project1"
- ✅ Session dropdown visible with "Select Session" placeholder
- ✅ "New Session" button visible

**Breadcrumb structure:** `Octobot / [project1] / [Select Session] / [New Session]`

---

### Scenario 5: Workspace and session both selected
**Setup:**
- `workspaces = [workspace1]`
- `selectedWorkspaceId = "ws-1"`
- `workspaceSessions = [session1, session2]`
- `selectedSession = session1`

**Expected UI:**
- ✅ Workspace dropdown shows "project1"
- ✅ Session dropdown shows "Session 1" (not "Select Session")
- ✅ Status indicator for session visible
- ✅ "New Session" button visible

**Breadcrumb structure:** `Octobot / [project1] / [Session 1] / [New Session]`

---

### Scenario 6: Session loading state
**Setup:**
- `workspaces = [workspace1]`
- `selectedWorkspaceId = "ws-1"`
- `isSessionLoading = true`
- `selectedSession = undefined`

**Expected UI:**
- ✅ "Loading..." text replaces workspace dropdown
- ✅ No session dropdown visible
- ✅ "New Session" button visible if workspaces exist

**Breadcrumb structure:** `Octobot / [Loading...] / [New Session]`

---

### Scenario 7: Workspace dropdown interactions
**Setup:**
- `workspaces = [workspace1, workspace2]`
- Open workspace dropdown

**Expected behavior:**
- ✅ All workspaces shown in dropdown
- ✅ Selected workspace indicated with checkmark
- ✅ "Add Workspace" menu item at bottom (with separator)
- ✅ Clicking workspace calls `showWorkspaceSessions(workspaceId)`
- ✅ Clicking "Add Workspace" opens workspace dialog

---

### Scenario 8: Session dropdown interactions
**Setup:**
- `workspaceSessions = [session1, session2]`
- `selectedSession = session1`
- Open session dropdown

**Expected behavior:**
- ✅ All sessions shown in dropdown
- ✅ Selected session indicated with checkmark
- ✅ Status indicator shown for each session
- ✅ "New Session" menu item at bottom (with separator)
- ✅ Clicking session calls `showSession(sessionId)`
- ✅ Delete button appears on hover
- ✅ Delete confirmation flow works

---

### Scenario 9: New Session button click
**Setup:**
- `selectedWorkspaceId = "ws-1"`
- Click "New Session" button

**Expected behavior:**
- ✅ Calls `showNewSession({ workspaceId: "ws-1" })`
- ✅ Button passes current workspace ID if one is selected
- ✅ Button passes undefined if no workspace selected

---

### Scenario 10: Workspace with displayName
**Setup:**
- `workspace = { id: "ws-2", displayName: "Project 2", path: "/home/user/project2" }`
- `selectedWorkspaceId = "ws-2"`

**Expected UI:**
- ✅ Shows "Project 2" (displayName) instead of "project2" (path-based name)

---

### Scenario 11: Workspace deletion
**Setup:**
- Open workspace dropdown
- Hover over workspace item
- Click delete button

**Expected behavior:**
- ✅ Delete button appears on hover
- ✅ Shows inline confirmation (check/cancel buttons)
- ✅ Confirming calls `deleteWorkspace(workspaceId)`
- ✅ If deleting current workspace, calls `showNewSession()`
- ✅ Canceling hides confirmation

---

### Scenario 12: Session deletion
**Setup:**
- Open session dropdown
- Hover over session item
- Click delete button

**Expected behavior:**
- ✅ Delete button appears on hover
- ✅ Shows inline confirmation (check/cancel buttons)
- ✅ Confirming calls `deleteSession(sessionId)`
- ✅ If deleting current session, calls `showNewSession()`
- ✅ Canceling hides confirmation

---

### Scenario 13: Breadcrumb separators
**Expected UI:**
- ✅ Forward slash `/` between Octobot and workspace dropdown
- ✅ Forward slash `/` between workspace and session (if session dropdown exists)
- ✅ Forward slash `/` before New Session button
- ✅ Consistent spacing using `gap-2` on flex container

---

### Scenario 14: Sidebar toggle
**Expected behavior:**
- ✅ Shows PanelLeftClose icon when `leftSidebarOpen={true}`
- ✅ Shows PanelLeft icon when `leftSidebarOpen={false}`
- ✅ Clicking toggle calls `onToggleSidebar()`

---

### Scenario 15: Tauri-specific controls (when IS_TAURI=true)
**Setup:**
- `IS_TAURI = true`
- `platform() = "macos"` or other

**Expected UI:**
- ✅ WindowControls rendered on left for macOS
- ✅ WindowControls rendered on right for Windows/Linux
- ✅ Buttons have `tauri-no-drag` class
- ✅ Header has `data-tauri-drag-region` attribute

---

### Scenario 16: Error states (session with error status)
**Setup:**
- `selectedSession.status = "error"`
- `selectedSession.errorMessage = "Error message"`

**Expected UI:**
- ✅ Session dropdown button shows error tooltip on hover
- ✅ Error status indicator visible
- ✅ Hover text displays error message

---

### Scenario 17: Failed commit status
**Setup:**
- `selectedSession.commitStatus = "failed"`
- `selectedSession.commitError = "Commit failed message"`

**Expected UI:**
- ✅ Session dropdown button shows failed commit tooltip on hover
- ✅ Failed commit status indicator visible
- ✅ Hover text displays commit error

---

## Implementation Checklist

### Conditional Rendering Logic:
- [x] Workspace dropdown always shows when not loading
- [x] Session dropdown only shows when `workspaceSessions.length > 0`
- [x] "New Session" button only shows when `workspaces.length > 0`
- [x] Loading state shows "Loading..." text

### Interaction Handlers:
- [x] `showNewSession({ workspaceId })` passes current workspace
- [x] `showWorkspaceSessions(workspaceId)` on workspace selection
- [x] `showSession(sessionId)` on session selection
- [x] `deleteWorkspace(workspaceId)` with confirmation
- [x] `deleteSession(sessionId)` with confirmation
- [x] Dialog opens for "Add Workspace"

### Visual Indicators:
- [x] Checkmarks on selected items
- [x] Status indicators for sessions
- [x] Delete buttons on hover
- [x] Forward slash separators
- [x] Dropdown chevrons

### Edge Cases:
- [x] Empty workspace list
- [x] Empty session list
- [x] No workspace selected
- [x] No session selected
- [x] Loading states
- [x] Error states
- [x] Failed commits

## Manual Testing Checklist

### Basic Navigation
- [ ] Click through all workspace options
- [ ] Click through all session options
- [ ] Click "New Session" button from various states
- [ ] Verify breadcrumb updates correctly

### Delete Operations
- [ ] Delete a workspace (non-selected)
- [ ] Delete the currently selected workspace
- [ ] Delete a session (non-selected)
- [ ] Delete the currently selected session
- [ ] Cancel delete operations

### Dropdown Interactions
- [ ] Open/close workspace dropdown
- [ ] Open/close session dropdown
- [ ] Click "Add Workspace"
- [ ] Click "New Session" from dropdown

### Edge Cases
- [ ] Start with no workspaces
- [ ] Add first workspace
- [ ] Create first session in workspace
- [ ] Switch between workspaces
- [ ] Switch between sessions
- [ ] Delete all sessions in workspace
- [ ] Delete all workspaces

### Visual Verification
- [ ] Forward slashes appear correctly
- [ ] Spacing is consistent
- [ ] Icons render properly
- [ ] Hover states work
- [ ] Status indicators show correctly
- [ ] Tooltips display for errors
