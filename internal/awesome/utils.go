package awesome

import (
	"fmt"
	"net/url"
	"strings"
)

// ExtractGitHubRepoFromURL extracts owner and repo name from a GitHub URL
func ExtractGitHubRepoFromURL(repoURL string) (owner, repo string, err error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Host != "github.com" {
		return "", "", fmt.Errorf("not a GitHub URL")
	}

	// Remove leading slash and split path
	path := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub URL format")
	}

	return parts[0], parts[1], nil
}
