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
