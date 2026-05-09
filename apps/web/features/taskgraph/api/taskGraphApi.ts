export const taskGraphApi = {
  getGraph: async (issueId: string) => {
    const res = await fetch(`/api/issues/${issueId}/graph`);
    return res.json();
  },

  createGraph: async (issueId: string, data: { nodes: any[]; edges: any[] }) => {
    const res = await fetch(`/api/issues/${issueId}/graph`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return res.json();
  },

  updateNode: async (id: string, data: any) => {
    const res = await fetch(`/api/graph/nodes/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });
    return res.json();
  },

  deleteNode: async (id: string) => {
    await fetch(`/api/graph/nodes/${id}`, { method: "DELETE" });
  },
};
