-- name: ListWorkspaces :many
SELECT w.* FROM workspace w
JOIN member m ON m.workspace_id = w.id
WHERE m.user_id = $1
ORDER BY w.created_at ASC;

-- name: GetWorkspace :one
SELECT * FROM workspace
WHERE id = $1;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspace
WHERE slug = $1;

-- name: CreateWorkspace :one
INSERT INTO workspace (name, slug, description, context, issue_prefix)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateWorkspace :one
UPDATE workspace SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    context = COALESCE(sqlc.narg('context'), context),
    settings = COALESCE(sqlc.narg('settings'), settings),
    repos = COALESCE(sqlc.narg('repos'), repos),
    issue_prefix = COALESCE(sqlc.narg('issue_prefix'), issue_prefix),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: IncrementIssueCounter :one
UPDATE workspace SET issue_counter = issue_counter + 1
WHERE id = $1
RETURNING issue_counter;

-- name: CountActiveMembers :one
SELECT count(*) FROM member
WHERE workspace_id = $1 AND invitation_status IN ('invited', 'active');

-- name: GetWorkspacePlan :one
SELECT id, plan, max_seats
FROM workspace
WHERE id = $1;

-- name: UpdateWorkspacePlan :one
UPDATE workspace SET
    plan = COALESCE(sqlc.narg('plan'), plan),
    max_seats = COALESCE(sqlc.narg('max_seats'), max_seats),
    updated_at = now()
WHERE id = $1
RETURNING id, plan, max_seats;

-- name: DeleteWorkspace :exec
DELETE FROM workspace WHERE id = $1;

-- name: ClaimWorkspaceDomain :one
UPDATE workspace
SET claimed_domain = $2, sso_policy = 'domain_claim', updated_at = now()
WHERE id = $1
RETURNING *;


-- name: GetSSOConfig :one
SELECT id, claimed_domain, sso_policy
FROM workspace
WHERE id = $1;

-- name: FindWorkspaceByEmailDomain :one
SELECT id, name, slug, claimed_domain, sso_policy
FROM workspace
WHERE claimed_domain = $1
LIMIT 1;
