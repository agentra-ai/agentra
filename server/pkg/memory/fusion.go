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

// Retrieve combines results from all retrievers using per-retriever ranks for RRF.
// Each retriever returns a sorted list; rank is the position within that list.
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
		WorkspaceID: opts.WorkspaceID,
		TimeRange:   timeRange,
	}

	// Run all retrievers in parallel
	var wg sync.WaitGroup
	type retrievalResult struct {
		name string
		mems []retrievers.Memory
		err  error
	}

	semanticCh := make(chan retrievalResult, 1)
	keywordCh := make(chan retrievalResult, 1)
	graphCh := make(chan retrievalResult, 1)
	temporalCh := make(chan retrievalResult, 1)

	wg.Add(4)

	go func() {
		defer wg.Done()
		mems, err := r.semantic.Retrieve(ctx, query, rops)
		semanticCh <- retrievalResult{name: "semantic", mems: mems, err: err}
	}()

	go func() {
		defer wg.Done()
		mems, err := r.keyword.Retrieve(ctx, query, rops)
		keywordCh <- retrievalResult{name: "keyword", mems: mems, err: err}
	}()

	go func() {
		defer wg.Done()
		mems, err := r.graph.Retrieve(ctx, query, rops)
		graphCh <- retrievalResult{name: "graph", mems: mems, err: err}
	}()

	go func() {
		defer wg.Done()
		mems, err := r.temporal.Retrieve(ctx, query, rops)
		temporalCh <- retrievalResult{name: "temporal", mems: mems, err: err}
	}()

	wg.Wait()
	close(semanticCh)
	close(keywordCh)
	close(graphCh)
	close(temporalCh)

	// Collect per-retriever result lists for rank-aware RRF
	var retrieverLists [][]retrievers.Memory
	for ch := range semanticCh {
		if ch.err == nil {
			retrieverLists = append(retrieverLists, ch.mems)
		}
	}
	for ch := range keywordCh {
		if ch.err == nil {
			retrieverLists = append(retrieverLists, ch.mems)
		}
	}
	for ch := range graphCh {
		if ch.err == nil {
			retrieverLists = append(retrieverLists, ch.mems)
		}
	}
	for ch := range temporalCh {
		if ch.err == nil {
			retrieverLists = append(retrieverLists, ch.mems)
		}
	}

	// Apply RRF fusion with proper per-retriever ranks
	fused := rrfFusion(retrieverLists, 60)

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

	if len(memoryResults) > limit {
		memoryResults = memoryResults[:limit]
	}

	return memoryResults, nil
}

// rrfFusion performs Reciprocal Rank Fusion across multiple retriever result lists.
// Each retriever's list is assumed to be sorted by relevance (best first).
// RRF score = Σ 1/(k + position_in_retriever_list), summed across all retrievers
// that returned this memory. k is typically 60.
func rrfFusion(perRetrieverResults [][]retrievers.Memory, k int) []retrievers.Memory {
	if len(perRetrieverResults) == 0 {
		return nil
	}

	// rrfScores accumulates RRF scores per memory ID.
	// bestMemory holds the highest-scoring instance of each memory.
	type entry struct {
		memory    retrievers.Memory
		rrfScore  float64
		seenFrom  int // number of retrievers that returned this memory
	}
	memoryMap := make(map[string]*entry)

	for _, retrieverList := range perRetrieverResults {
		for pos, mem := range retrieverList {
			rank := pos + 1 // 1-based rank within this retriever
			e, exists := memoryMap[mem.ID]
			if !exists {
				memoryMap[mem.ID] = &entry{
					memory:   mem,
					rrfScore: 1.0 / float64(rank+k),
					seenFrom: 1,
				}
			} else {
				e.rrfScore += 1.0 / float64(rank+k)
				e.seenFrom++
				// Keep the best-scoring instance (from whichever retriever ranked it highest)
				if mem.Score > e.memory.Score {
					e.memory = mem
				}
			}
		}
	}

	// Flatten and sort by RRF score descending
	result := make([]retrievers.Memory, 0, len(memoryMap))
	for _, e := range memoryMap {
		e.memory.Score = e.rrfScore
		result = append(result, e.memory)
	}

	sort.Slice(result, func(i, j int) bool {
		// Tie-break: prefer memories found by more retrievers
		if result[i].Score == result[j].Score {
			return memoryMap[result[i].ID].seenFrom > memoryMap[result[j].ID].seenFrom
		}
		return result[i].Score > result[j].Score
	})

	return result
}

// SemanticRetriever performs vector similarity search using pgvector.
type SemanticRetriever struct {
	pool     *pgxpool.Pool
	embedder *EmbeddingClient
}

func NewSemanticRetriever(pool *pgxpool.Pool, embedder *EmbeddingClient) *SemanticRetriever {
	return &SemanticRetriever{pool: pool, embedder: embedder}
}

func (r *SemanticRetriever) Name() string {
	return "semantic"
}

func (r *SemanticRetriever) Retrieve(ctx context.Context, query string, opts retrievers.RetrieveOptions) ([]retrievers.Memory, error) {
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	limit := opts.Limit
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

	wsUUID, err := uuid.Parse(opts.WorkspaceID)
	if err != nil {
		wsUUID = uuid.Nil
	}

	params := db.SearchAgentMemoriesParams{
		AgentID:     uuidToPg(agentUUID),
		Column2:     vectorToPg(vec),
		WorkspaceID: uuidToPg(wsUUID),
		Column4:     memTypes,
		Limit:       int32(limit),
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
