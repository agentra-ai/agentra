package github

import (
	"context"

	"github.com/google/go-github/v67/github"
)

// Client wraps the GitHub client with additional functionality.
type Client struct {
	api *github.Client
}

// NewClient creates a new GitHub API client.
func NewClient(token string) *Client {
	return &Client{
		api: github.NewTokenClient(token),
	}
}

// ListRepositories returns repositories for the authenticated user.
func (c *Client) ListRepositories(ctx context.Context) ([]*github.Repository, error) {
	opts := &github.ListOptions{PerPage: 100}
	repos, _, err := c.api.Repositories.List(ctx, "", opts)
	return repos, err
}