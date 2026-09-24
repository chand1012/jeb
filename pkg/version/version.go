package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// Version is the current version of the application
	// This is set via ldflags during build
	Version = "dev"

	// CommitHash is the git commit hash of the current build
	// This is set via ldflags during build
	CommitHash = "unknown"

	// BuildDate is the date when the binary was built
	// This is set via ldflags during build
	BuildDate = "unknown"

	// The GitHub API URL for the latest release
	githubAPIURL = "https://api.github.com/repos/chand1012/jeb/releases/latest"
)

// ReleaseInfo represents the GitHub release information
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// CheckVersion checks if the current version is the latest
// Returns true if an update is available, along with the latest version and any error
func CheckVersion() (bool, string, error) {
	// If we're running a dev build, skip version check
	if Version == "dev" {
		return false, "", nil
	}

	// Create a new HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Make request to GitHub API
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent header as required by GitHub API
	req.Header.Set("User-Agent", "OpenRules-Version-Checker")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest release: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return false, "", fmt.Errorf("failed to parse release info: %w", err)
	}

	// Clean version strings (remove 'v' prefix if present)
	currentVersion := strings.TrimPrefix(Version, "v")
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// Compare versions
	// If current version doesn't match latest version, an update is available
	updateAvailable := currentVersion != latestVersion

	return updateAvailable, release.TagName, nil
}

// GetVersionInfo returns a string containing the current version, commit hash, and build date
func GetVersionInfo() string {
	if Version == "dev" {
		return fmt.Sprintf("Version: dev (commit: %s, built: %s)", CommitHash, BuildDate)
	}
	return fmt.Sprintf("Version: %s (commit: %s, built: %s)", Version, CommitHash, BuildDate)
}
