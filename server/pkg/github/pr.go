package github

// PRState represents the state of a pull request.
type PRState string

const (
	PRStateOpen   PRState = "open"
	PRStateClosed PRState = "closed"
	PRStateMerged PRState = "merged"
)

// PRReview represents a pull request review.
type PRReview struct {
	State string
	Body  string
}