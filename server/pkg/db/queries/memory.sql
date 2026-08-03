-- name: CreateAgentMemory :one
INSERT INTO agent_memories (agent_id, workspace_id, memory_type, content, embedding, metadata, is_private)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAgentMemories :many
SELECT * FROM agent_memories
WHERE agent_id = $1
ORDER BY created_at DESC;

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

-- name: DeleteAgentMemory :one
DELETE FROM agent_memories WHERE id = $1 AND agent_id = $2 RETURNING *;

-- name: CreateTeamMemory :one
INSERT INTO team_memory (workspace_id, memory_type, content, embedding, metadata, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTeamMemories :many
SELECT * FROM team_memory WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: DeleteTeamMemory :one
DELETE FROM team_memory WHERE id = $1 AND workspace_id = $2 RETURNING *;
