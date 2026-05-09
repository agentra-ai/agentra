package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v67/github"
)

type App struct {
	appID      int64
	privateKey []byte
	client     *github.Client
}

func NewApp(appID int64, privateKey []byte) *App {
	return &App{
		appID:      appID,
		privateKey: privateKey,
	}
}

// InstallForRepo returns an authenticated client for a specific installation.
func (a *App) InstallForRepo(ctx context.Context, installationID int64) (*github.Client, error) {
	token, _, err := a.client.Apps.ObtainInstallationToken(ctx, installationID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain installation token: %w", err)
	}

	client := github.NewTokenClient(token.GetToken())
	return client, nil
}

// CreatePR creates a pull request in the specified repository.
func (a *App) CreatePR(ctx context.Context, client *github.Client, owner, repo string, pr *PROptions) (*PR, error) {
	newPR := &github.NewPullRequest{
		Title:               github.String(pr.Title),
		Head:                github.String(pr.Head),
		Base:                github.String(pr.Base),
		Body:                github.String(pr.Body),
		MaintainerCanModify: github.Bool(true),
	}

	prResult, _, err := client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("failed to create PR: %w", err)
	}

	return &PR{
		Number: prResult.GetNumber(),
		URL:    prResult.GetHTMLURL(),
		State:  prResult.GetState(),
	}, nil
}

type PROptions struct {
	Title string
	Head  string
	Base  string
	Body  string
}

type PR struct {
	Number int
	URL    string
	State  string
}