package github

import (
	"context"
	"fmt"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/google/uuid"
)

type SyncService struct {
	queries *db.Queries
}

func NewSyncService(queries *db.Queries) *SyncService {
	return &SyncService{queries: queries}
}

func (s *SyncService) LinkIssueToPR(ctx context.Context, issueID, repo string, prNumber int) error {
	issueUUID, err := uuid.Parse(issueID)
	if err != nil {
		return fmt.Errorf("invalid issue id: %w", err)
	}
	_, err = s.queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
		IssueID:    issueUUID,
		Repository: repo,
		PrNumber:   int64(prNumber),
	})
	return err
}

func (s *SyncService) UpdatePRStatusForIssue(ctx context.Context, issueID string, status string) error {
	issueUUID, err := uuid.Parse(issueID)
	if err != nil {
		return fmt.Errorf("invalid issue id: %w", err)
	}
	links, err := s.queries.GetIssueLinks(ctx, issueUUID)
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
