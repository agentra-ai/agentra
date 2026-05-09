package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
)

type WebhookHandler struct {
	secret  []byte
	handler WebhookEventHandler
}

type WebhookEventHandler interface {
	HandlePREvent(ctx context.Context, pr *PRPayload) error
	HandlePushEvent(ctx context.Context, push *PushPayload) error
	HandleCommentEvent(ctx context.Context, comment *CommentPayload) error
}

func NewWebhookHandler(secret string, h WebhookEventHandler) *WebhookHandler {
	return &WebhookHandler{secret: []byte(secret), handler: h}
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", 500)
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !wh.verifySignature(payload, signature) {
		http.Error(w, "Invalid signature", 401)
		return
	}

	event := r.Header.Get("X-GitHub-Event")

	switch event {
	case "pull_request":
		var pr PRPayload
		json.Unmarshal(payload, &pr)
		wh.handler.HandlePREvent(r.Context(), &pr)
	case "push":
		var push PushPayload
		json.Unmarshal(payload, &push)
		wh.handler.HandlePushEvent(r.Context(), &push)
	case "issue_comment":
		var comment CommentPayload
		json.Unmarshal(payload, &comment)
		wh.handler.HandleCommentEvent(r.Context(), &comment)
	}

	w.WriteHeader(200)
}

func (wh *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, wh.secret)
	mac.Write(payload)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

type PRPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title    string `json:"title"`
		State    string `json:"state"`
		HTMLURL  string `json:"html_url"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type PushPayload struct {
	Ref        string   `json:"ref"`
	Commits    []Commit `json:"commits"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

type CommentPayload struct {
	Action string `json:"action"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
	Issue struct {
		Number int `json:"number"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}