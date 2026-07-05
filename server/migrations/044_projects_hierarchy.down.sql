DROP INDEX IF EXISTS idx_issue_project;
ALTER TABLE issue DROP COLUMN IF EXISTS project_id;
DROP TABLE IF EXISTS milestones;
DROP INDEX IF EXISTS idx_milestones_project;
DROP TABLE IF EXISTS projects;
DROP INDEX IF EXISTS idx_projects_workspace;
