package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CIProvider interface for CI/CD integrations (GitHub, GitLab, etc.)
type CIProvider interface {
	// Name returns the provider name
	Name() string
	// IsAvailable checks if the provider CLI is installed and authenticated
	IsAvailable() bool
	// GetPRInfo fetches PR/MR info for a branch
	GetPRInfo(branch string) (*PRInfo, error)
	// GetCIStatus fetches CI status for a branch
	GetCIStatus(branch string) (*CIInfo, error)
	// OpenPR opens the PR/MR in browser
	OpenPR(branch string) error
	// GetBranchForPRNumber returns the head branch name for a PR/MR number
	GetBranchForPRNumber(number int) (string, error)
}

// ParsePRRef parses a "pr:<number>" or "mr:<number>" reference.
// Returns the prefix ("pr" or "mr"), the number, and an error if the ref is invalid.
func ParsePRRef(ref string) (prefix string, number int, err error) {
	var numStr string
	switch {
	case strings.HasPrefix(ref, "pr:"):
		prefix = "pr"
		numStr = strings.TrimPrefix(ref, "pr:")
	case strings.HasPrefix(ref, "mr:"):
		prefix = "mr"
		numStr = strings.TrimPrefix(ref, "mr:")
	default:
		return "", 0, fmt.Errorf("%q is not a pr:/mr: reference", ref)
	}

	n, parseErr := strconv.Atoi(numStr)
	if parseErr != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid %s reference %q: number must be a positive integer", prefix, ref)
	}
	return prefix, n, nil
}

// IsPRRef returns true if the string is a valid pr:/mr: reference.
func IsPRRef(ref string) bool {
	_, _, err := ParsePRRef(ref)
	return err == nil
}

// PRInfo contains pull/merge request information
type PRInfo struct {
	Number  int    `json:"number"`
	State   string `json:"state"` // OPEN, CLOSED, MERGED
	URL     string `json:"url"`
	IsDraft bool   `json:"isDraft"`
}

// CIInfo contains CI status information
type CIInfo struct {
	Status     string `json:"status"`     // success, failure, pending
	Conclusion string `json:"conclusion"` // Detailed conclusion
	URL        string `json:"url"`        // URL to checks page
}

// DetectProvider detects and returns the appropriate CI provider
func DetectProvider() CIProvider {
	// Check remote URL to detect provider
	remoteURL := getRemoteURL()

	if strings.Contains(remoteURL, "gitlab") {
		return &GitLabProvider{}
	}

	// Default to GitHub
	return &GitHubProvider{}
}

// getRemoteURL returns the git remote URL
func getRemoteURL() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// GitHubProvider implements CIProvider for GitHub
type GitHubProvider struct{}

func (g *GitHubProvider) Name() string {
	return "github"
}

func (g *GitHubProvider) IsAvailable() bool {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run() == nil
}

func (g *GitHubProvider) GetPRInfo(branch string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "number,state,url,isDraft")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pr PRInfo
	if err := json.Unmarshal(output, &pr); err != nil {
		return nil, err
	}

	return &pr, nil
}

func (g *GitHubProvider) GetCIStatus(branch string) (*CIInfo, error) {
	cmd := exec.Command("gh", "pr", "checks", branch, "--json", "state,name,conclusion")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var checks []struct {
		State      string `json:"state"`
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal(output, &checks); err != nil {
		return nil, err
	}

	if len(checks) == 0 {
		return nil, nil
	}

	info := &CIInfo{}
	hasFailure := false
	hasPending := false
	allSuccess := true

	for _, check := range checks {
		switch check.State {
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

	if hasFailure {
		info.Status = "failure"
		info.Conclusion = "Some checks failed"
	} else if hasPending {
		info.Status = "pending"
		info.Conclusion = "Checks in progress"
	} else if allSuccess {
		info.Status = "success"
		info.Conclusion = "All checks passed"
	} else {
		info.Status = "unknown"
	}

	return info, nil
}

func (g *GitHubProvider) OpenPR(branch string) error {
	cmd := exec.Command("gh", "pr", "view", branch, "--web")
	return cmd.Run()
}

func (g *GitHubProvider) GetBranchForPRNumber(number int) (string, error) {
	cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", number), "--json", "headRefName")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch PR #%d: %w", number, err)
	}

	var result struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse PR response: %w", err)
	}
	if result.HeadRefName == "" {
		return "", fmt.Errorf("PR #%d has no head branch", number)
	}
	return result.HeadRefName, nil
}

// GitLabProvider implements CIProvider for GitLab
type GitLabProvider struct {
	host string // For self-hosted GitLab instances
}

func (g *GitLabProvider) Name() string {
	return "gitlab"
}

func (g *GitLabProvider) IsAvailable() bool {
	// Check if glab is installed
	if _, err := exec.LookPath("glab"); err != nil {
		return false
	}
	// Check if glab is authenticated
	cmd := exec.Command("glab", "auth", "status")
	return cmd.Run() == nil
}

func (g *GitLabProvider) GetPRInfo(branch string) (*PRInfo, error) {
	// GitLab uses "merge requests" (MR) not "pull requests"
	cmd := exec.Command("glab", "mr", "view", branch, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var mr struct {
		IID    int    `json:"iid"`
		State  string `json:"state"` // opened, closed, merged
		WebURL string `json:"web_url"`
		Draft  bool   `json:"draft"`
	}
	if err := json.Unmarshal(output, &mr); err != nil {
		return nil, err
	}

	// Map GitLab states to our normalized states
	state := strings.ToUpper(mr.State)
	switch state {
	case "OPENED":
		state = "OPEN"
	}

	return &PRInfo{
		Number:  mr.IID,
		State:   state,
		URL:     mr.WebURL,
		IsDraft: mr.Draft,
	}, nil
}

func (g *GitLabProvider) GetCIStatus(branch string) (*CIInfo, error) {
	// Get pipeline status for the branch
	cmd := exec.Command("glab", "ci", "status", "-b", branch, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pipelines []struct {
		Status string `json:"status"` // pending, running, success, failed, canceled
		WebURL string `json:"web_url"`
	}
	if err := json.Unmarshal(output, &pipelines); err != nil {
		// Try single pipeline format
		var pipeline struct {
			Status string `json:"status"`
			WebURL string `json:"web_url"`
		}
		if err := json.Unmarshal(output, &pipeline); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}

	if len(pipelines) == 0 {
		return nil, nil
	}

	// Get the latest pipeline
	latest := pipelines[0]

	info := &CIInfo{URL: latest.WebURL}

	switch latest.Status {
	case "success":
		info.Status = "success"
		info.Conclusion = "Pipeline passed"
	case "failed":
		info.Status = "failure"
		info.Conclusion = "Pipeline failed"
	case "pending", "running", "created":
		info.Status = "pending"
		info.Conclusion = "Pipeline in progress"
	case "canceled":
		info.Status = "failure"
		info.Conclusion = "Pipeline canceled"
	default:
		info.Status = "unknown"
		info.Conclusion = fmt.Sprintf("Pipeline status: %s", latest.Status)
	}

	return info, nil
}

func (g *GitLabProvider) OpenPR(branch string) error {
	cmd := exec.Command("glab", "mr", "view", branch, "--web")
	return cmd.Run()
}

func (g *GitLabProvider) GetBranchForPRNumber(number int) (string, error) {
	cmd := exec.Command("glab", "mr", "view", fmt.Sprintf("%d", number), "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch MR !%d: %w", number, err)
	}

	var mr struct {
		SourceBranch string `json:"source_branch"`
	}
	if err := json.Unmarshal(output, &mr); err != nil {
		return "", fmt.Errorf("failed to parse MR response: %w", err)
	}
	if mr.SourceBranch == "" {
		return "", fmt.Errorf("MR !%d has no source branch", number)
	}
	return mr.SourceBranch, nil
}
