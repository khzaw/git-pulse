package remote

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
)

type Provider string

const (
	ProviderUnknown Provider = ""
	ProviderGitHub  Provider = "github"
	ProviderGitLab  Provider = "gitlab"
)

type RepositoryRef struct {
	Provider Provider
	Owner    string
	Name     string
}

func (r RepositoryRef) FullName() string {
	if r.Owner == "" || r.Name == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

func Detect(path string) (RepositoryRef, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return RepositoryRef{}, fmt.Errorf("open repository: %w", err)
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return RepositoryRef{}, fmt.Errorf("list remotes: %w", err)
	}

	var urls []string
	for _, remote := range remotes {
		if remote.Config().Name == "origin" {
			if ref, ok := DetectFromURLs(remote.Config().URLs); ok {
				return ref, nil
			}
		}
		urls = append(urls, remote.Config().URLs...)
	}

	if ref, ok := DetectFromURLs(urls); ok {
		return ref, nil
	}
	return RepositoryRef{}, nil
}

func DetectFromURLs(urls []string) (RepositoryRef, bool) {
	for _, raw := range urls {
		if ref, ok := parseRemoteURL(raw); ok {
			return ref, true
		}
	}
	return RepositoryRef{}, false
}

func parseRemoteURL(raw string) (RepositoryRef, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RepositoryRef{}, false
	}

	if strings.HasPrefix(raw, "git@") {
		host, path, ok := parseSCPStyle(raw)
		if !ok {
			return RepositoryRef{}, false
		}
		return repositoryRefFromParts(host, path)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return RepositoryRef{}, false
	}
	return repositoryRefFromParts(parsed.Hostname(), parsed.Path)
}

func parseSCPStyle(raw string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimPrefix(raw, "git@"), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func repositoryRefFromParts(host, path string) (RepositoryRef, bool) {
	path = strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".git")
	segments := strings.Split(filepath.ToSlash(path), "/")
	if len(segments) < 2 {
		return RepositoryRef{}, false
	}

	ref := RepositoryRef{
		Owner: segments[len(segments)-2],
		Name:  segments[len(segments)-1],
	}
	switch host {
	case "github.com", "www.github.com":
		ref.Provider = ProviderGitHub
	case "gitlab.com", "www.gitlab.com":
		ref.Provider = ProviderGitLab
	default:
		return RepositoryRef{}, false
	}
	return ref, true
}
