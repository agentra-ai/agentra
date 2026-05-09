package memory

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

type EmbeddingClient struct {
	client  *openai.Client
	model   string
	dim     int
}

func NewEmbeddingClient() *EmbeddingClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := os.Getenv("EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &EmbeddingClient{
		client: openai.NewClient(apiKey),
		model:  model,
		dim:    1536,
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingCreateInput{
		Input: text,
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, ErrNoEmbedding
	}
	// text-embedding-3-small returns 1536-dim vectors
	vec := make([]float32, c.dim)
	copy(vec, resp.Data[0].Embedding)
	return vec, nil
}

func (c *EmbeddingClient) Dim() int { return c.dim }

var ErrNoEmbedding = fmt.Errorf("no embedding returned")
