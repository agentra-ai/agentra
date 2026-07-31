export interface Project {
  id: string;
  workspace_id: string;
  title: string;
  slug: string;
  owner_id: string;
  deadline: string | null;
  created_at: string;
  updated_at: string;
}

export interface Milestone {
  id: string;
  project_id: string;
  title: string;
  deadline: string | null;
  status: string;
  created_at: string;
  updated_at: string;
}
