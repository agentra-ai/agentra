-- name: CreateTaskNode :one
INSERT INTO task_graph_nodes (workspace_id, issue_id, agent_id, node_type, status, context, depth)
VALUES ($1, $2, $3, $4, 'pending', $5, $6)
RETURNING *;

-- name: GetTaskNode :one
SELECT * FROM task_graph_nodes WHERE id = $1;

-- name: ListNodesByIssue :many
SELECT * FROM task_graph_nodes WHERE issue_id = $1 ORDER BY depth, created_at;

-- name: UpdateTaskNode :one
UPDATE task_graph_nodes SET
    agent_id = COALESCE(sqlc.narg('agent_id'), agent_id),
    status = COALESCE(sqlc.narg('status'), status),
    context = COALESCE(sqlc.narg('context'), context),
    result = COALESCE(sqlc.narg('result'), result),
    position_x = COALESCE(sqlc.narg('position_x'), position_x),
    position_y = COALESCE(sqlc.narg('position_y'), position_y),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateTaskEdge :one
INSERT INTO task_graph_edges (from_node_id, to_node_id, edge_type, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEdgesByIssue :many
SELECT DISTINCT e.* FROM task_graph_edges e
JOIN task_graph_nodes n ON n.id = e.from_node_id OR n.id = e.to_node_id
WHERE n.issue_id = $1
ORDER BY e.created_at;

-- name: DeleteTaskNode :one
DELETE FROM task_graph_nodes WHERE id = $1 RETURNING *;

-- name: DeleteTaskEdge :one
DELETE FROM task_graph_edges WHERE id = $1 RETURNING *;
