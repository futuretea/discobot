package oauth

// NewGitHubProvider creates a device flow provider for GitHub git operations.
// Uses repo + read:user + user:email scopes for cloning private repos, pushing, and creating PRs.
// If domain is empty, defaults to github.com.
func NewGitHubProvider(clientID, domain string) *GitHubCopilotProvider {
	if domain == "" {
		domain = DefaultGitHubDomain
	}
	return &GitHubCopilotProvider{
		ClientID: clientID,
		Domain:   domain,
		Scopes:   []string{"repo", "read:user", "user:email"},
	}
}
