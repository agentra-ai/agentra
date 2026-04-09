export interface IssueTemplate {
  id: string;
  label: string;
  description: string;
  defaultTitle?: string;
  defaultDescription?: string;
  defaultPriority: "urgent" | "high" | "medium" | "low";
}

export const ISSUE_TEMPLATES: IssueTemplate[] = [
  {
    id: "feature",
    label: "Feature",
    description: "New functionality request",
    defaultPriority: "medium",
  },
  {
    id: "bug",
    label: "Bug",
    description: "Something isn't working as expected",
    defaultPriority: "high",
  },
  {
    id: "refactor",
    label: "Refactor",
    description: "Code structure improvement without behavior change",
    defaultPriority: "low",
  },
  {
    id: "agent-task",
    label: "Agent Task",
    description: "Task for an AI agent to execute",
    defaultPriority: "medium",
  },
];
