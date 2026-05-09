-- name: CreateAgentMemory :one
INSERT INTO agent_memories (agent_id, workspace_id, memory_type, content, embedding, metadata, is_private)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAgentMemories :many
SELECT * FROM agent_memories
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: SearchAgentMemories :many
SELECT id, agent_id, memory_type, content, metadata, is_private, created_at,
       (1 - (embedding <=> $2::vector))::float AS score
FROM agent_memories
WHERE agent_id = $1 AND workspace_id = $3
  AND ($4::text[] IS NULL OR memory_type = ANY($4))
ORDER BY embedding <=> $2::vector
LIMIT $5;

-- name: SearchAgentMemoriesBM25 :many
SELECT id, agent_id, memory_type, content, metadata, is_private, created_at,
       ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) AS score
FROM agent_memories
WHERE agent_id = $1 AND workspace_id = $3
  AND to_tsvector('english', content) @@ plainto_tsquery('english', $2)
  AND ($4::text[] IS NULL OR memory_type = ANY($4))
ORDER BY score DESC
LIMIT $5;

-- name: SearchAllMemoriesBM25 :many
SELECT id, agent_id, memory_type, content, metadata, created_at,
       ts_rank(to_tsvector('english', content), plainto_tsquery('english', $2)) AS score
FROM (
    SELECT id, agent_id, memory_type, content, metadata, created_at
    FROM agent_memories
    WHERE agent_memories.workspace_id = $1 AND is_private = false
    UNION ALL
    SELECT id, NULL, memory_type, content, metadata, created_at
    FROM team_memory
    WHERE team_memory.workspace_id = $1
) AS all_memories
WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $2)
  AND ($3::text[] IS NULL OR memory_type = ANY($3))
ORDER BY score DESC
LIMIT $4;

-- name: ListAgentMemoriesTimeRange :many
SELECT id, agent_id, memory_type, content, metadata, is_private, created_at
FROM agent_memories
WHERE agent_id = $1 AND workspace_id = $2
  AND ($3::text[] IS NULL OR memory_type = ANY($3))
  AND created_at BETWEEN $4 AND $5
ORDER BY created_at DESC
LIMIT $6;

-- name: ListAllMemoriesTimeRange :many
SELECT am.id, am.agent_id, am.memory_type, am.content, am.metadata, am.created_at
FROM agent_memories am
WHERE am.workspace_id = $1 AND am.is_private = false
  AND ($2::text[] IS NULL OR am.memory_type = ANY($2))
  AND am.created_at BETWEEN $3 AND $4
UNION ALL
SELECT tm.id, NULL, tm.memory_type, tm.content, tm.metadata, tm.created_at
FROM team_memory tm
WHERE tm.workspace_id = $1
  AND ($2::text[] IS NULL OR tm.memory_type = ANY($2))
  AND tm.created_at BETWEEN $3 AND $4
ORDER BY created_at DESC
LIMIT $5;

-- name: DeleteAgentMemory :one
DELETE FROM agent_memories WHERE id = $1 AND agent_id = $2 RETURNING *;

-- name: UpdateAgentMemory :one
UPDATE agent_memories SET
    content = COALESCE(sqlc.narg('content'), content),
    memory_type = COALESCE(sqlc.narg('memory_type'), memory_type),
    is_private = COALESCE(sqlc.narg('is_private'), is_private),
    embedding = COALESCE(sqlc.narg('embedding'), embedding),
    updated_at = now()
WHERE id = $1 AND agent_id = $2 AND workspace_id = $3
RETURNING *;

-- name: CreateTeamMemory :one
INSERT INTO team_memory (workspace_id, memory_type, content, embedding, metadata, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTeamMemories :many
SELECT * FROM team_memory WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: SearchTeamMemories :many
SELECT id, memory_type, content, metadata, created_by, created_at,
       (1 - (embedding <=> $2::vector))::float AS score
FROM team_memory
WHERE workspace_id = $1
  AND ($3::text[] IS NULL OR memory_type = ANY($3))
ORDER BY embedding <=> $2::vector
LIMIT $4;

-- name: DeleteTeamMemory :one
DELETE FROM team_memory WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: UpdateTeamMemory :one
UPDATE team_memory SET
    content = COALESCE(sqlc.narg('content'), content),
    memory_type = COALESCE(sqlc.narg('memory_type'), memory_type),
    embedding = COALESCE(sqlc.narg('embedding'), embedding),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: SearchAllMemories :many
SELECT id, agent_id, memory_type, content, metadata, created_at,
       (1 - (embedding <=> $2::vector))::float AS score
FROM (
    SELECT id, agent_id, memory_type, content, metadata, created_at, embedding
    FROM agent_memories
    WHERE agent_memories.workspace_id = $1 AND is_private = false
    UNION ALL
    SELECT id, NULL, memory_type, content, metadata, created_at, embedding
    FROM team_memory
    WHERE team_memory.workspace_id = $1
) AS all_memories
WHERE $3::text[] IS NULL OR memory_type = ANY($3)
ORDER BY embedding <=> $2::vector
LIMIT $4;

-- name: ListMemoriesGraph :many
-- Entity/temporal/causal graph traversal via entity extraction and relationship queries
-- This query finds memories containing entities mentioned in the query and related entities
SELECT am.id, am.agent_id, am.memory_type, am.content, am.metadata, am.is_private, am.created_at
FROM agent_memories am
WHERE am.workspace_id = $1
  AND ($2::text[] IS NULL OR am.memory_type = ANY($2))
  AND (
    -- Primary entity matches (entities extracted from query)
    am.content ILIKE '%' || $3 || '%'
    -- Related entity matches (entities found in existing memories)
    OR am.id IN (
      SELECT DISTINCT am2.id
      FROM agent_memories am2
      WHERE am2.workspace_id = $1
        AND am2.content ILIKE '%' || $4 || '%'
    )
  )
ORDER BY am.created_at DESC
LIMIT $5;