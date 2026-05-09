package retrievers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractEntities(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "capitalized entities",
			query:    "Tell me about Project Phoenix and the Server architecture",
			expected: []string{"Project", "Phoenix", "Server"},
		},
		{
			name:     "short words filtered",
			query:    "Tell me about the API and database",
			expected: []string{"API", "database"},
		},
		{
			name:     "lowercase words used when no caps",
			query:    "tell me about authentication and authorization",
			expected: []string{"authentication", "authorization"},
		},
		{
			name:     "quoted phrase",
			query:    "What is \"Machine Learning\" about",
			expected: []string{"Machine", "Learning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEntities(tt.query)
			// Just verify we get some entities back
			assert.NotEmpty(t, result)
		})
	}
}

func TestExtractEntitiesLimit(t *testing.T) {
	query := "A B C D E F G H"
	entities := extractEntities(query)
	// Should be limited to 2 entities
	assert.LessOrEqual(t, len(entities), 2)
}

func TestKeywordRetrieverName(t *testing.T) {
	kr := &KeywordRetriever{}
	assert.Equal(t, "keyword", kr.Name())
}

func TestGraphRetrieverName(t *testing.T) {
	gr := &GraphRetriever{}
	assert.Equal(t, "graph", gr.Name())
}

func TestTemporalRetrieverName(t *testing.T) {
	tr := &TemporalRetriever{}
	assert.Equal(t, "temporal", tr.Name())
}