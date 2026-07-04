export interface IssueTemplate {
  id: string;
  defaultTitle?: string;
  defaultDescription?: string;
  defaultPriority: "urgent" | "high" | "medium" | "low";
}

export const ISSUE_TEMPLATES: IssueTemplate[] = [
  {
    id: "feature",
    defaultPriority: "medium",
  },
  {
    id: "bug",
    defaultPriority: "high",
  },
  {
    id: "refactor",
    defaultPriority: "low",
  },
  {
    id: "agentTask",
    defaultPriority: "medium",
  },
];
