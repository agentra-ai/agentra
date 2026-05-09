-- 034_task_graph.down.sql
DROP INDEX IF EXISTS task_graph_edges_to_id_idx;
DROP INDEX IF EXISTS task_graph_edges_from_id_idx;
DROP TABLE IF EXISTS task_graph_edges;
DROP INDEX IF EXISTS task_graph_nodes_workspace_id_idx;
DROP INDEX IF EXISTS task_graph_nodes_issue_id_idx;
DROP TABLE IF EXISTS task_graph_nodes;
