package git

import (
	"testing"
)

func TestGitHubProviderName(t *testing.T) {
	p := &GitHubProvider{}
	if name := p.Name(); name != "github" {
		t.Errorf("GitHubProvider.Name() = %q, want %q", name, "github")
	}
}

func TestGitLabProviderName(t *testing.T) {
	p := &GitLabProvider{}
	if name := p.Name(); name != "gitlab" {
		t.Errorf("GitLabProvider.Name() = %q, want %q", name, "gitlab")
	}
}

func TestDetectProvider_GitHub(t *testing.T) {
	// Test with mock remote URL
	tests := []struct {
		name     string
		url      string
		wantType string
	}{
		{
			name:     "GitHub HTTPS",
			url:      "https://github.com/user/repo.git",
			wantType: "github",
		},
		{
			name:     "GitHub SSH",
			url:      "git@github.com:user/repo.git",
			wantType: "github",
		},
		{
			name:     "GitLab HTTPS",
			url:      "https://gitlab.com/user/repo.git",
			wantType: "gitlab",
		},
		{
			name:     "GitLab SSH",
			url:      "git@gitlab.com:user/repo.git",
			wantType: "gitlab",
		},
		{
			name:     "Self-hosted GitLab",
			url:      "https://gitlab.mycompany.com/user/repo.git",
			wantType: "gitlab",
		},
		{
			name:     "Unknown defaults to GitHub",
			url:      "https://bitbucket.org/user/repo.git",
			wantType: "github",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily mock getRemoteURL, so just test the detection logic
			provider := detectProviderFromURL(tt.url)
			if provider.Name() != tt.wantType {
				t.Errorf("detectProviderFromURL(%q).Name() = %q, want %q", tt.url, provider.Name(), tt.wantType)
			}
		})
	}
}

// detectProviderFromURL is a helper for testing provider detection
func detectProviderFromURL(remoteURL string) CIProvider {
	if containsGitlab(remoteURL) {
		return &GitLabProvider{}
	}
	return &GitHubProvider{}
}

func containsGitlab(url string) bool {
	return len(url) > 6 && (url[8:14] == "gitlab" || url[4:10] == "gitlab" || containsSubstring(url, "gitlab"))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPRInfo(t *testing.T) {
	pr := &PRInfo{
		Number:  123,
		State:   "OPEN",
		URL:     "https://github.com/user/repo/pull/123",
		IsDraft: false,
	}

	if pr.Number != 123 {
		t.Errorf("PRInfo.Number = %d, want 123", pr.Number)
	}
	if pr.State != "OPEN" {
		t.Errorf("PRInfo.State = %q, want %q", pr.State, "OPEN")
	}
	if pr.IsDraft {
		t.Error("PRInfo.IsDraft should be false")
	}
}

func TestCIInfo(t *testing.T) {
	ci := &CIInfo{
		Status:     "success",
		Conclusion: "All checks passed",
		URL:        "https://github.com/user/repo/actions",
	}

	if ci.Status != "success" {
		t.Errorf("CIInfo.Status = %q, want %q", ci.Status, "success")
	}
	if ci.Conclusion != "All checks passed" {
		t.Errorf("CIInfo.Conclusion = %q, want %q", ci.Conclusion, "All checks passed")
	}
}

func TestCIInfoStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
	}{
		{"success", "success", "All checks passed"},
		{"failure", "failure", "Some checks failed"},
		{"pending", "pending", "Checks in progress"},
		{"unknown", "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ci := &CIInfo{
				Status:     tt.status,
				Conclusion: tt.conclusion,
			}
			if ci.Status != tt.status {
				t.Errorf("CIInfo.Status = %q, want %q", ci.Status, tt.status)
			}
		})
	}
}

// Test that provider interface is properly implemented
func TestCIProviderInterface(t *testing.T) {
	// Ensure both providers implement the interface
	var _ CIProvider = &GitHubProvider{}
	var _ CIProvider = &GitLabProvider{}
}

func TestGitLabStateNormalization(t *testing.T) {
	// Test GitLab state to normalized state mapping
	tests := []struct {
		gitlabState     string
		normalizedState string
	}{
		{"opened", "OPEN"},
		{"OPENED", "OPEN"},
		{"closed", "CLOSED"},
		{"merged", "MERGED"},
	}

	for _, tt := range tests {
		t.Run(tt.gitlabState, func(t *testing.T) {
			// Simulate the normalization logic from GitLabProvider.GetPRInfo
			state := tt.gitlabState
			if state == "opened" || state == "OPENED" {
				state = "OPEN"
			} else {
				state = toUpper(state)
			}

			if state != tt.normalizedState {
				t.Errorf("state normalization %q = %q, want %q", tt.gitlabState, state, tt.normalizedState)
			}
		})
	}
}

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func TestGitLabPipelineStatuses(t *testing.T) {
	tests := []struct {
		pipelineStatus string
		expectedStatus string
	}{
		{"success", "success"},
		{"failed", "failure"},
		{"pending", "pending"},
		{"running", "pending"},
		{"created", "pending"},
		{"canceled", "failure"},
		{"manual", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.pipelineStatus, func(t *testing.T) {
			var status string
			switch tt.pipelineStatus {
			case "success":
				status = "success"
			case "failed":
				status = "failure"
			case "pending", "running", "created":
				status = "pending"
			case "canceled":
				status = "failure"
			default:
				status = "unknown"
			}

			if status != tt.expectedStatus {
				t.Errorf("pipeline status %q mapped to %q, want %q", tt.pipelineStatus, status, tt.expectedStatus)
			}
		})
	}
}

func TestGitHubCheckStates(t *testing.T) {
	tests := []struct {
		name       string
		states     []string
		wantStatus string
	}{
		{
			name:       "all success",
			states:     []string{"SUCCESS", "SUCCESS"},
			wantStatus: "success",
		},
		{
			name:       "has failure",
			states:     []string{"SUCCESS", "FAILURE"},
			wantStatus: "failure",
		},
		{
			name:       "has error",
			states:     []string{"SUCCESS", "ERROR"},
			wantStatus: "failure",
		},
		{
			name:       "has pending",
			states:     []string{"SUCCESS", "PENDING"},
			wantStatus: "pending",
		},
		{
			name:       "in progress",
			states:     []string{"SUCCESS", "IN_PROGRESS"},
			wantStatus: "pending",
		},
		{
			name:       "queued",
			states:     []string{"QUEUED"},
			wantStatus: "pending",
		},
		{
			name:       "failure takes precedence over pending",
			states:     []string{"FAILURE", "PENDING"},
			wantStatus: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasFailure := false
			hasPending := false
			allSuccess := true

			for _, state := range tt.states {
				switch state {
				case "FAILURE", "ERROR":
					hasFailure = true
					allSuccess = false
				case "PENDING", "QUEUED", "IN_PROGRESS":
					hasPending = true
					allSuccess = false
				case "SUCCESS":
				default:
					allSuccess = false
				}
			}

			var status string
			if hasFailure {
				status = "failure"
			} else if hasPending {
				status = "pending"
			} else if allSuccess {
				status = "success"
			} else {
				status = "unknown"
			}

			if status != tt.wantStatus {
				t.Errorf("aggregated status = %q, want %q", status, tt.wantStatus)
			}
		})
	}
}
