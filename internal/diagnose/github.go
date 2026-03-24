package diagnose

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// GitHubCollector gathers repo statistics from GitHub.
type GitHubCollector struct {
	token string
	user  string
}

// NewGitHubCollector creates a GitHub stats collector.
func NewGitHubCollector(token, user string) *GitHubCollector {
	return &GitHubCollector{token: token, user: user}
}

// CollectStats returns aggregate GitHub repo metrics.
func (g *GitHubCollector) CollectStats(ctx context.Context) (GitHubStats, error) {
	client := github.NewClient(nil)
	if g.token != "" {
		client = client.WithAuthToken(g.token)
	}

	var stats GitHubStats
	opts := &github.RepositoryListByUserOptions{
		Type:        "owner",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := client.Repositories.ListByUser(ctx, g.user, opts)
		if err != nil {
			return stats, fmt.Errorf("list repos: %w", err)
		}

		for _, repo := range repos {
			stats.RepoCount++
			stats.TotalStars += repo.GetStargazersCount()
			stats.TotalForks += repo.GetForksCount()
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return stats, nil
}
