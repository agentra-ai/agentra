package github

import (
	"context"

	"github.com/google/uuid"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type SyncService struct {
	queries *db.Queries
}

func NewSyncService(queries *db.Queries) *SyncService {
	return &SyncService{queries: queries}
}

func (s *SyncService) LinkIssueToPR(ctx context.Context, issueID, repo string, prNumber int) error {
	_, err := s.queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
		IssueID:   uuid.MustParse(issueID),
		Repository: repo,
		PrNumber:   int64(prNumber),
	})
	return err
}

func (s *SyncService) UpdatePRStatusForIssue(ctx context.Context, issueID string, status string) error {
	links, err := s.queries.GetIssueLinks(ctx, uuid.MustParse(issueID))
	if err != nil {
		return err
	}

	// Update each linked PR's status
	for _, link := range links {
		if link.PrNumber != nil {
			_ = status // TODO: Call GitHub API to update PR status
		}
	}
	return nil
}