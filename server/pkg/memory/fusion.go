package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/pkg/memory/retrievers"
)

// FusionRetriever combines multiple retrieval strategies using Reciprocal Rank Fusion.
type FusionRetriever struct {
	semantic  *SemanticRetriever
	keyword   *retrievers.KeywordRetriever
	graph     *retrievers.GraphRetriever
	temporal  *retrievers.TemporalRetriever
	embedder  *EmbeddingClient
	pool      *pgxpool.Pool
}

// NewFusionRetriever creates a new fusion retriever combining all strategies.
func NewFusionRetriever(pool *pgxpool.Pool, embedder *EmbeddingClient) *FusionRetriever {
	return &FusionRetriever{
		semantic:  NewSemanticRetriever(pool, embedder),
		keyword:   retrievers.NewKeywordRetriever(pool),
		graph:     retrievers.NewGraphRetriever(pool),
		temporal:  retrievers.NewTemporalRetriever(pool),
		embedder:  embedder,
		pool:      pool,
	}
}

// Name returns the identifier for this retriever.
func (r *FusionRetriever) Name() string {
	return "fusion"
}

// Retrieve combines results from all retrievers using RRF.
func (r *FusionRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	// Convert to retrievers.RetrieveOptions for sub-retrievers
	memTypes := make([]retrievers.MemoryType, len(opts.MemoryTypes))
	for i, t := range opts.MemoryTypes {
		memTypes[i] = retrievers.MemoryType(t)
	}
	var timeRange *retrievers.TimeRange
	if opts.TimeRange != nil {
		timeRange = &retrievers.TimeRange{
			Start: opts.TimeRange.Start,
			End:   opts.TimeRange.End,
		}
	}
	rops := retrievers.RetrieveOptions{
		Limit:       opts.Limit,
		MemoryTypes: memTypes,
		AgentID:     opts.AgentID,
		TimeRange:   timeRange,
	}

	// Run all retrievers in parallel
	var wg sync.WaitGroup
	type retrievalResult struct {
		mems []retrievers.Memory
		err  error
	}

	semanticCh := make(chan retrievalResult, 1)
	keywordCh := make(chan retrievalResult, 1)
	graphCh := make(chan retrievalResult, 1)
	temporalCh := make(chan retrievalResult, 1)

	wg.Add(4)

	// Semantic retrieval
	go func() {
		defer wg.Done()
		mems, err := r.semantic.Retrieve(ctx, query, rops)
		semanticCh <- retrievalResult{mems: mems, err: err}
	}()

	// Keyword retrieval
	go func() {
		defer wg.Done()
		mems, err := r.keyword.Retrieve(ctx, query, rops)
		keywordCh <- retrievalResult{mems: mems, err: err}
	}()

	// Graph retrieval
	go func() {
		defer wg.Done()
		mems, err := r.graph.Retrieve(ctx, query, rops)
		graphCh <- retrievalResult{mems: mems, err: err}
	}()

	// Temporal retrieval
	go func() {
		defer wg.Done()
		mems, err := r.temporal.Retrieve(ctx, query, rops)
		temporalCh <- retrievalResult{mems: mems, err: err}
	}()

	wg.Wait()

	close(semanticCh)
	close(keywordCh)
	close(graphCh)
	close(temporalCh)

	// Collect results
	var allResults []retrievers.Memory

	for ch := range semanticCh {
		if ch.err == nil {
			allResults = append(allResults, ch.mems...)
		}
	}
	for ch := range keywordCh {
		if ch.err == nil {
			allResults = append(allResults, ch.mems...)
		}
	}
	for ch := range graphCh {
		if ch.err == nil {
			allResults = append(allResults, ch.mems...)
		}
	}
	for ch := range temporalCh {
		if ch.err == nil {
			allResults = append(allResults, ch.mems...)
		}
	}

	// Apply RRF fusion
	fused := rrfFusion(allResults, 60) // standard k=60 for RRF

	// Convert back to memory.Memory
	memoryResults := make([]Memory, len(fused))
	for i, m := range fused {
		memoryResults[i] = Memory{
			ID:         m.ID,
			MemoryType: MemoryType(m.MemoryType),
			Content:    m.Content,
			AgentID:    m.AgentID,
			Score:      m.Score,
			Source:     m.Source,
		}
	}

	// Limit results
	if len(memoryResults) > limit {
		memoryResults = memoryResults[:limit]
	}

	return memoryResults, nil
}

// rrfFusion performs Reciprocal Rank Fusion on a list of memories from multiple sources.
// k is the ranking constant (typically 60).
func rrfFusion(memories []retrievers.Memory, k int) []retrievers.Memory {
	if len(memories) == 0 {
		return nil
	}

	// Group by ID
	type memEntry struct {
		memory  retrievers.Memory
		rank    int
	}
	idToEntries := make(map[string][]memEntry)
	idToBestMemory := make(map[string]retrievers.Memory)

	for _, mem := range memories {
		if _, exists := idToBestMemory[mem.ID]; !exists {
			idToBestMemory[mem.ID] = mem
		}
		// Track all ranks for this memory
		idToEntries[mem.ID] = append(idToEntries[mem.ID], memEntry{memory: mem, rank: len(idToEntries[mem.ID])})
	}

	// Calculate RRF score for each memory
	type scoredMem struct {
		memory  retrievers.Memory
		score   float64
	}
	var scored []scoredMem

	for id, entries := range idToEntries {
		var rrfScore float64
		for _, entry := range entries {
			rrfScore += 1.0 / float64(entry.rank+k)
		}
		scored = append(scored, scoredMem{
			memory: idToBestMemory[id],
			score:  rrfScore,
		})
	}

	// Sort by RRF score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	result := make([]retrievers.Memory, len(scored))
	for i, s := range scored {
		result[i] = s.memory
		result[i].Score = s.score
	}

	return result
}

// SemanticRetriever performs vector similarity search using pgvector.
type SemanticRetriever struct {
	pool     *pgxpool.Pool
	embedder *EmbeddingClient
}

// NewSemanticRetriever creates a new semantic (vector) retriever.
func NewSemanticRetriever(pool *pgxpool.Pool, embedder *EmbeddingClient) *SemanticRetriever {
	return &SemanticRetriever{pool: pool, embedder: embedder}
}

// Name returns the identifier for this retriever.
func (r *SemanticRetriever) Name() string {
	return "semantic"
}

// Retrieve performs vector similarity search on memory content.
func (r *SemanticRetriever) Retrieve(ctx context.Context, query string, opts retrievers.RetrieveOptions) ([]retrievers.Memory, error) {
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	limit := int32(opts.Limit)
	if limit <= 0 {
		limit = 20
	}

	memTypes := make([]string, len(opts.MemoryTypes))
	for i, t := range opts.MemoryTypes {
		memTypes[i] = string(t)
	}

	queries := db.New(r.pool)

	agentUUID, err := uuid.Parse(opts.AgentID)
	if err != nil {
		agentUUID = uuid.Nil
	}

	workspaceID := uuid.Nil // TODO: get from context

	params := db.SearchAgentMemoriesParams{
		AgentID:     uuidToPg(agentUUID),
		Column2:     vectorToPg(vec),
		WorkspaceID: uuidToPg(workspaceID),
		Column4:     memTypes,
		Limit:       limit,
	}

	rows, err := queries.SearchAgentMemories(ctx, params)
	if err != nil {
		return nil, err
	}

	mems := make([]retrievers.Memory, 0, len(rows))
	for _, row := range rows {
		mems = append(mems, retrievers.Memory{
			ID:         row.ID.String(),
			MemoryType: retrievers.MemoryType(row.MemoryType),
			Content:    row.Content,
			AgentID:    row.AgentID.String(),
			Score:      row.Score,
			Source:     "semantic",
		})
	}

	return mems, nil
}