package integration

import (
	"net/http"
	"testing"
)

func TestGetAgentTypes(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/agents/types")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		AgentTypes []map[string]interface{} `json:"agentTypes"`
	}
	ParseJSON(t, resp, &result)

	if len(result.AgentTypes) == 0 {
		t.Error("Expected at least one agent type")
	}

	// Check that claude-code is in the list
	found := false
	for _, agentType := range result.AgentTypes {
		if agentType["id"] == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'claude-code' agent type")
	}
}

func TestListAgents_Empty(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/agents")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Agents []interface{} `json:"agents"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(result.Agents))
	}
}

func TestCreateAgent(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/agents", map[string]interface{}{
		"agentType": "claude-code",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusCreated)

	var agent map[string]interface{}
	ParseJSON(t, resp, &agent)

	if agent["agentType"] != "claude-code" {
		t.Errorf("Expected agentType 'claude-code', got '%v'", agent["agentType"])
	}
	// Verify ID is set
	if agent["id"] == nil || agent["id"] == "" {
		t.Error("Expected agent ID to be set")
	}
}

func TestCreateAgent_MissingType(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/agents", map[string]interface{}{})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestGetAgent(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	agent := ts.CreateTestAgent(project, "Test Agent", "claude-code")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/agents/" + agent.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]interface{}
	ParseJSON(t, resp, &result)

	if result["id"] != agent.ID {
		t.Errorf("Expected id '%s', got '%v'", agent.ID, result["id"])
	}
	if result["agentType"] != "claude-code" {
		t.Errorf("Expected agentType 'claude-code', got '%v'", result["agentType"])
	}
}

func TestUpdateAgent(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	agent := ts.CreateTestAgent(project, "Test Agent", "claude-code")
	client := ts.AuthenticatedClient(user)

	// Update agent - currently this just returns the agent (no fields are updateable)
	resp := client.Put("/api/projects/"+project.ID+"/agents/"+agent.ID, map[string]interface{}{})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]interface{}
	ParseJSON(t, resp, &result)

	// Verify agent is returned correctly
	if result["id"] != agent.ID {
		t.Errorf("Expected id '%s', got '%v'", agent.ID, result["id"])
	}
	if result["agentType"] != "claude-code" {
		t.Errorf("Expected agentType 'claude-code', got '%v'", result["agentType"])
	}
}

func TestDeleteAgent(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	agent := ts.CreateTestAgent(project, "Test Agent", "claude-code")
	client := ts.AuthenticatedClient(user)

	resp := client.Delete("/api/projects/" + project.ID + "/agents/" + agent.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	// Verify agent is deleted
	resp = client.Get("/api/projects/" + project.ID + "/agents/" + agent.ID)
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusNotFound)
}

func TestSetDefaultAgent(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	agent := ts.CreateTestAgent(project, "Test Agent", "claude-code")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/agents/default", map[string]string{
		"agentId": agent.ID,
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result map[string]bool
	ParseJSON(t, resp, &result)

	if !result["success"] {
		t.Error("Expected success to be true")
	}
}

func TestSetDefaultAgent_MissingAgentId(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	resp := client.Post("/api/projects/"+project.ID+"/agents/default", map[string]string{})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusBadRequest)
}

func TestListAgents_WithData(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	ts.CreateTestAgent(project, "Claude Agent", "claude-code")
	ts.CreateTestAgent(project, "Aider Agent", "aider")
	client := ts.AuthenticatedClient(user)

	resp := client.Get("/api/projects/" + project.ID + "/agents")
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Agents []interface{} `json:"agents"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(result.Agents))
	}
}

func TestCreateAgent_FirstAgentIsDefault(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	// Create first agent
	resp := client.Post("/api/projects/"+project.ID+"/agents", map[string]interface{}{
		"agentType": "claude-code",
	})
	defer resp.Body.Close()

	AssertStatus(t, resp, http.StatusCreated)

	var agent map[string]interface{}
	ParseJSON(t, resp, &agent)

	// Verify first agent is set as default
	if agent["isDefault"] != true {
		t.Error("Expected first agent to be set as default")
	}
}

func TestSetDefaultAgent_UnsetsExistingDefault(t *testing.T) {
	t.Parallel()
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	client := ts.AuthenticatedClient(user)

	// Create first agent (will be default)
	resp := client.Post("/api/projects/"+project.ID+"/agents", map[string]interface{}{
		"agentType": "claude-code",
	})
	defer resp.Body.Close()
	AssertStatus(t, resp, http.StatusCreated)

	var agent1 map[string]interface{}
	ParseJSON(t, resp, &agent1)
	agent1ID := agent1["id"].(string)

	// Create second agent
	resp = client.Post("/api/projects/"+project.ID+"/agents", map[string]interface{}{
		"agentType": "aider",
	})
	defer resp.Body.Close()
	AssertStatus(t, resp, http.StatusCreated)

	var agent2 map[string]interface{}
	ParseJSON(t, resp, &agent2)
	agent2ID := agent2["id"].(string)

	// Verify second agent is not default
	if agent2["isDefault"] == true {
		t.Error("Expected second agent to not be default")
	}

	// Set second agent as default
	resp = client.Post("/api/projects/"+project.ID+"/agents/default", map[string]string{
		"agentId": agent2ID,
	})
	defer resp.Body.Close()
	AssertStatus(t, resp, http.StatusOK)

	// List all agents and verify only agent2 is default
	resp = client.Get("/api/projects/" + project.ID + "/agents")
	defer resp.Body.Close()
	AssertStatus(t, resp, http.StatusOK)

	var result struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	ParseJSON(t, resp, &result)

	if len(result.Agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(result.Agents))
	}

	// Count defaults
	defaultCount := 0
	for _, agent := range result.Agents {
		if agent["isDefault"] == true {
			defaultCount++
			if agent["id"] != agent2ID {
				t.Errorf("Expected agent2 to be default, but agent %s is default", agent["id"])
			}
		}
		if agent["id"] == agent1ID && agent["isDefault"] == true {
			t.Error("Expected agent1 to no longer be default")
		}
	}

	if defaultCount != 1 {
		t.Errorf("Expected exactly 1 default agent, got %d", defaultCount)
	}
}
