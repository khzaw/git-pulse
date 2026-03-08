package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var conventionalCommitPattern = regexp.MustCompile(`^([a-z]+)(\([^)]+\))?!?:\s+`)

type RepositoryData struct {
	Path          string
	DefaultBranch string
	Head          string
	Commits       []CommitRecord
	Branches      []BranchRecord
	Tags          []TagRecord
}

type CommitRecord struct {
	Hash             string
	AuthorName       string
	AuthorEmail      string
	When             time.Time
	Subject          string
	ConventionalType string
	Additions        int
	Deletions        int
	Files            []FileStat
}

type FileStat struct {
	Path      string
	Additions int
	Deletions int
}

type BranchRecord struct {
	Name         string
	Hash         string
	LastCommitAt time.Time
}

type TagRecord struct {
	Name string
	Hash string
	When time.Time
}

func Scan(ctx context.Context, path string) (RepositoryData, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return RepositoryData{}, fmt.Errorf("open repository: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return RepositoryData{}, fmt.Errorf("resolve HEAD: %w", err)
	}

	commits, err := collectCommits(ctx, path, repo, headRef.Hash())
	if err != nil {
		return RepositoryData{}, err
	}
	branches, err := collectBranches(ctx, repo)
	if err != nil {
		return RepositoryData{}, err
	}
	tags, err := collectTags(ctx, repo)
	if err != nil {
		return RepositoryData{}, err
	}

	return RepositoryData{
		Path:          path,
		DefaultBranch: headRef.Name().Short(),
		Head:          headRef.Hash().String(),
		Commits:       commits,
		Branches:      branches,
		Tags:          tags,
	}, nil
}

func collectCommits(ctx context.Context, repoPath string, repo *git.Repository, head plumbing.Hash) ([]CommitRecord, error) {
	commits, err := collectCommitsFromGit(ctx, repoPath)
	if err == nil {
		return commits, nil
	}
	return collectCommitsFromGoGit(ctx, repoPath, repo, head)
}

func collectCommitsFromGoGit(ctx context.Context, repoPath string, repo *git.Repository, head plumbing.Hash) ([]CommitRecord, error) {
	iter, err := repo.Log(&git.LogOptions{From: head})
	if err != nil {
		return nil, fmt.Errorf("iterate commits: %w", err)
	}
	defer iter.Close()

	var commits []CommitRecord
	err = iter.ForEach(func(commit *object.Commit) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		stats, err := commitStats(ctx, repoPath, commit)
		if err != nil {
			return fmt.Errorf("collect stats for %s: %w", commit.Hash.String(), err)
		}

		record := CommitRecord{
			Hash:             commit.Hash.String(),
			AuthorName:       commit.Author.Name,
			AuthorEmail:      commit.Author.Email,
			When:             commit.Author.When.UTC(),
			Subject:          strings.TrimSpace(commit.Message),
			ConventionalType: conventionalType(commit.Message),
		}
		for _, stat := range stats {
			record.Additions += stat.Addition
			record.Deletions += stat.Deletion
			record.Files = append(record.Files, FileStat{
				Path:      stat.Name,
				Additions: stat.Addition,
				Deletions: stat.Deletion,
			})
		}
		commits = append(commits, record)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(commits, func(i, j int) bool {
		return commits[i].When.Before(commits[j].When)
	})
	return commits, nil
}

func collectCommitsFromGit(ctx context.Context, repoPath string) ([]CommitRecord, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "log", "--date=iso-strict", "--numstat", "--format=%x1e%H%x1f%an%x1f%ae%x1f%aI%x1f%s", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	commits, err := parseGitLogOutput(output)
	if err != nil {
		return nil, err
	}
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].When.Before(commits[j].When)
	})
	return commits, nil
}

func collectBranches(ctx context.Context, repo *git.Repository) ([]BranchRecord, error) {
	iter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	defer iter.Close()

	var branches []BranchRecord
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return fmt.Errorf("branch %s commit: %w", ref.Name().Short(), err)
		}
		branches = append(branches, BranchRecord{
			Name:         ref.Name().Short(),
			Hash:         ref.Hash().String(),
			LastCommitAt: commit.Author.When.UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].LastCommitAt.After(branches[j].LastCommitAt)
	})
	return branches, nil
}

func collectTags(ctx context.Context, repo *git.Repository) ([]TagRecord, error) {
	iter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer iter.Close()

	var tags []TagRecord
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		hash := ref.Hash()
		when := time.Time{}
		if tag, err := repo.TagObject(hash); err == nil {
			hash = tag.Target
			when = tag.Tagger.When.UTC()
		}

		commit, err := repo.CommitObject(hash)
		if err != nil {
			return fmt.Errorf("tag %s commit: %w", ref.Name().Short(), err)
		}
		if when.IsZero() {
			when = commit.Author.When.UTC()
		}

		tags = append(tags, TagRecord{
			Name: ref.Name().Short(),
			Hash: hash.String(),
			When: when,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i].When.Before(tags[j].When) })
	return tags, nil
}

func conventionalType(message string) string {
	firstLine := strings.TrimSpace(strings.Split(message, "\n")[0])
	match := conventionalCommitPattern.FindStringSubmatch(strings.ToLower(firstLine))
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func commitStats(ctx context.Context, repoPath string, commit *object.Commit) (object.FileStats, error) {
	stats, err := commit.Stats()
	if err == nil {
		return stats, nil
	}
	return commitStatsFromGit(ctx, repoPath, commit.Hash.String())
}

func commitStatsFromGit(ctx context.Context, repoPath, hash string) (object.FileStats, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "show", "--numstat", "--format=", hash, "--")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseNumStat(output), nil
}

func parseNumStat(output []byte) object.FileStats {
	lines := bytes.Split(output, []byte{'\n'})
	stats := make(object.FileStats, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}

		additions, addErr := parseNumStatValue(fields[0])
		deletions, delErr := parseNumStatValue(fields[1])
		if addErr != nil || delErr != nil {
			continue
		}

		stats = append(stats, object.FileStat{
			Name:     fields[2],
			Addition: additions,
			Deletion: deletions,
		})
	}
	return stats
}

func parseNumStatValue(value string) (int, error) {
	if value == "-" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseGitLogOutput(output []byte) ([]CommitRecord, error) {
	records := bytes.Split(output, []byte{0x1e})
	commits := make([]CommitRecord, 0, len(records))

	for _, record := range records {
		record = bytes.TrimSpace(record)
		if len(record) == 0 {
			continue
		}

		lines := bytes.Split(record, []byte{'\n'})
		header := strings.Split(string(lines[0]), "\x1f")
		if len(header) != 5 {
			return nil, fmt.Errorf("parse git log header: unexpected field count %d", len(header))
		}

		when, err := time.Parse(time.RFC3339, header[3])
		if err != nil {
			return nil, fmt.Errorf("parse git log time %q: %w", header[3], err)
		}

		commit := CommitRecord{
			Hash:             header[0],
			AuthorName:       header[1],
			AuthorEmail:      header[2],
			When:             when.UTC(),
			Subject:          header[4],
			ConventionalType: conventionalType(header[4]),
		}

		stats := parseNumStat(bytes.Join(lines[1:], []byte{'\n'}))
		for _, stat := range stats {
			commit.Additions += stat.Addition
			commit.Deletions += stat.Deletion
			commit.Files = append(commit.Files, FileStat{
				Path:      stat.Name,
				Additions: stat.Addition,
				Deletions: stat.Deletion,
			})
		}

		commits = append(commits, commit)
	}

	return commits, nil
}
