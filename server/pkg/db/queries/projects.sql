-- name: CreateProject :one
INSERT INTO projects (workspace_id, title, slug, owner_id, deadline)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1;

-- name: GetProjectBySlug :one
SELECT * FROM projects
WHERE workspace_id = $1 AND slug = $2;

-- name: ListProjectsByWorkspace :many
SELECT * FROM projects
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateProject :one
UPDATE projects SET
    title = COALESCE(sqlc.narg('title'), title),
    slug = COALESCE(sqlc.narg('slug'), slug),
    deadline = COALESCE(sqlc.narg('deadline'), deadline),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: CreateMilestone :one
INSERT INTO milestones (project_id, title, deadline, status)
VALUES ($1, $2, $3, COALESCE(sqlc.narg('status'), 'active'))
RETURNING *;

-- name: ListMilestonesByProject :many
SELECT * FROM milestones
WHERE project_id = $1
ORDER BY created_at ASC;

-- name: UpdateMilestoneStatus :one
UPDATE milestones SET
    title = COALESCE(sqlc.narg('title'), title),
    status = COALESCE(sqlc.narg('status'), status),
    deadline = COALESCE(sqlc.narg('deadline'), deadline),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMilestone :exec
DELETE FROM milestones WHERE id = $1;

-- name: AssignIssueToProject :one
UPDATE issue SET project_id = $2, updated_at = now()
WHERE id = $1 AND workspace_id = $3
RETURNING *;

-- name: RemoveIssueFromProject :one
UPDATE issue SET project_id = NULL, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ListIssuesByProject :many
SELECT * FROM issue
WHERE project_id = $1
ORDER BY position ASC, created_at DESC;

-- name: ListIssuesByProjectWithLimit :many
SELECT * FROM issue
WHERE project_id = $1
ORDER BY position ASC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetProjectWithIssueCount :one
SELECT p.*,
       (SELECT count(*) FROM issue i WHERE i.project_id = p.id) AS issue_count
FROM projects p
WHERE p.id = $1;
