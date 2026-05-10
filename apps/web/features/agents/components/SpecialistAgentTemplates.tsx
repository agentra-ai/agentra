"use client";

import { useState } from "react";
import { Bot, Code, TestTube, FileText, Shield, Globe, Database, Wrench, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "sonner";
import { api } from "@/shared/api";

interface SpecialistTemplate {
  id: string;
  name: string;
  description: string;
  icon: typeof Code;
  instructions: string;
  suggested_tools: string[];
}

const SPECIALIST_TEMPLATES: SpecialistTemplate[] = [
  {
    id: "frontend-engineer",
    name: "Frontend Engineer",
    description: "Specializes in React, TypeScript, and CSS. Writes clean, testable components.",
    icon: Code,
    instructions: `You are an expert frontend engineer specializing in React, TypeScript, and modern CSS.

## Working Style
- Write small, focused PRs — one commit per logical change
- Use TypeScript strictly; avoid 'any' types
- Follow the existing component patterns in features/
- Use shadcn/ui components when available
- Prefer composition over inheritance

## Technical Focus
- React 18 with App Router patterns
- Zustand for client state, React Context only for connection lifecycle
- Tailwind CSS with shadcn design tokens
- No hardcoded colors — use design tokens (bg-primary, text-muted-foreground, etc.)

## Quality
- Add unit tests for new components using Vitest
- Run pnpm typecheck before committing
- Ensure pnpm build succeeds before marking ready

## Constraints
- Do not modify shared/ types without explicit approval
- Keep API calls in shared/api.ts, not in components
- Use @/ alias for imports`,
    suggested_tools: ["GitHub", "Playwright", "npm"],
  },
  {
    id: "backend-engineer",
    name: "Backend Engineer",
    description: "Go expert with PostgreSQL, API design, and system reliability focus.",
    icon: Database,
    instructions: `You are an expert backend engineer specializing in Go, PostgreSQL, and API design.

## Working Style
- Write small, focused PRs with clear commit messages
- Follow Go idioms: error wrapping, context propagation, interface design
- Keep handlers thin; push logic into service layers
- Use sqlc for all database queries

## Technical Focus
- Go with Chi router, pgx/v5 for PostgreSQL
- RESTful API design with proper status codes
- Background jobs with structured logging via slog
- Environment-based config (no hardcoded values)

## Quality
- Run go vet and go test ./... before committing
- Add integration tests for new handler paths
- Ensure DB migrations are reversible

## Constraints
- Never commit secrets or API keys
- Use transaction patterns for multi-table operations
- Add proper context cancellation to all DB operations`,
    suggested_tools: ["PostgreSQL", "Docker", "GitHub"],
  },
  {
    id: "test-engineer",
    name: "Test Engineer",
    description: "QA-focused agent that writes comprehensive tests and finds edge cases.",
    icon: TestTube,
    instructions: `You are a meticulous test engineer focused on finding edge cases and ensuring quality.

## Working Style
- Write tests BEFORE or alongside implementation
- Cover happy path AND edge cases and error scenarios
- Use property-based testing where appropriate

## Frontend Testing
- Vitest for unit tests, Playwright for E2E
- Mock external dependencies and third-party services
- Test user interactions as the user would experience them

## Backend Testing
- go test for unit and integration tests
- Use test database fixtures, not mocks
- Test at the handler level with httptest

## Coverage Goals
- Aim for >80% coverage on new code
- Never decrease overall coverage
- Add regression tests for any bug fixed

## Constraints
- Tests must be deterministic (no flaky tests)
- E2E tests must clean up after themselves
- Do not commit commented-out test code`,
    suggested_tools: ["Playwright", "Vitest", "GitHub Actions"],
  },
  {
    id: "security-engineer",
    name: "Security Engineer",
    description: "OWASP expert focused on finding and fixing security vulnerabilities.",
    icon: Shield,
    instructions: `You are a security expert specializing in OWASP Top 10 and secure coding practices.

## Focus Areas
- Input validation and sanitization
- Authentication and authorization flaws
- SQL injection and query parameterization
- XSS and CSRF prevention
- Secrets management and encryption

## Code Review
- Review PRs for security issues before merge
- Check that all user input is validated
- Verify SQL queries use parameterized statements
- Ensure auth checks are on all protected endpoints

## Testing
- Use static analysis tools (go vet, eslint)
- Test edge cases in auth flows
- Verify CORS and CSP headers are configured

## When Finding Issues
- Report with severity (Critical/High/Medium/Low)
- Provide reproduction steps
- Suggest concrete fixes with code examples

## Constraints
- Never store passwords in plain text
- Never log sensitive data (tokens, passwords, PII)
- Always use HTTPS in production`,
    suggested_tools: ["SAST", "DAST", "Secret Scanner"],
  },
  {
    id: "devops-engineer",
    name: "DevOps Engineer",
    description: "Infrastructure, CI/CD, and deployment automation expert.",
    icon: Wrench,
    instructions: `You are a DevOps engineer focused on reliable deployments and infrastructure.

## Working Style
- Automate everything that can be automated
- Prefer immutable infrastructure
- Document infrastructure as code

## Focus Areas
- Docker and containerization
- CI/CD pipelines (GitHub Actions)
- PostgreSQL backup and recovery
- Monitoring and alerting (slog, health checks)
- Zero-downtime deployments

## Infrastructure
- Use Docker Compose for local development
- Ensure health check endpoints for all services
- Set up structured logging with slog
- Configure rate limiting and timeouts

## Deployment Safety
- Use migration scripts that are backward compatible
- Implement graceful shutdown for long-running operations
- Set up rollback mechanisms for failed deployments

## Constraints
- Never commit production secrets
- Use environment variables for all config
- Ensure all services have health endpoints`,
    suggested_tools: ["Docker", "GitHub Actions", "AWS", "PostgreSQL"],
  },
  {
    id: "technical-writer",
    name: "Technical Writer",
    description: "Creates clear documentation, API docs, and user guides.",
    icon: FileText,
    instructions: `You are a technical writer focused on clear, concise documentation.

## Writing Style
- Use active voice and short sentences
- Include code examples for all API endpoints
- Structure docs with headings, lists, and tables
- Write for the reader's level of expertise

## Documentation Types
- README files with setup instructions
- API documentation with examples
- User guides with step-by-step instructions
- Architecture decision records (ADRs)
- Inline code comments for complex logic

## Quality
- Review docs for accuracy before publishing
- Keep docs up-to-date with code changes
- Use consistent terminology throughout

## Tools
- Markdown for documentation
- Include diagrams where helpful
- Generate API docs from code comments

## Constraints
- Never leave TODO comments in final code
- Remove commented-out code from PRs
- Keep docs in the repository near the code they describe`,
    suggested_tools: ["Markdown", "OpenAPI", "Mermaid"],
  },
];

interface CreateFromTemplateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  runtimes: { id: string }[];
  onCreated: (agentId: string) => void;
}

export function CreateFromTemplateDialog({ open, onOpenChange, runtimes, onCreated }: CreateFromTemplateDialogProps) {
  const [selectedTemplate, setSelectedTemplate] = useState<SpecialistTemplate | null>(null);
  const [customName, setCustomName] = useState("");
  const [creating, setCreating] = useState(false);

  const handleSelectTemplate = (template: SpecialistTemplate) => {
    setSelectedTemplate(template);
    setCustomName(template.name);
  };

  const handleCreate = async () => {
    if (!selectedTemplate || !customName.trim() || runtimes.length === 0) return;

    setCreating(true);
    try {
      const agent = await api.createAgent({
        name: customName.trim(),
        description: selectedTemplate.description,
        instructions: selectedTemplate.instructions,
        runtime_id: runtimes[0]!.id,
        visibility: "workspace",
        triggers: [
          { id: `${Date.now()}-1`, type: "on_assign", enabled: true, config: {} },
          { id: `${Date.now()}-2`, type: "on_comment", enabled: true, config: {} },
        ],
      });
      toast.success(`${selectedTemplate.name} agent created`);
      onCreated(agent.id);
      onOpenChange(false);
      setSelectedTemplate(null);
      setCustomName("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create agent");
    } finally {
      setCreating(false);
    }
  };

  const handleClose = (open: boolean) => {
    if (!open) {
      setSelectedTemplate(null);
      setCustomName("");
    }
    onOpenChange(open);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-2xl max-h-[80vh] overflow-y-auto">
        {!selectedTemplate ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Sparkles className="h-4 w-4" />
                Create from Template
              </DialogTitle>
              <DialogDescription>
                Choose a specialist template to快速创建一个专业AI代理
              </DialogDescription>
            </DialogHeader>

            <div className="grid grid-cols-2 gap-3 py-4">
              {SPECIALIST_TEMPLATES.map((template) => {
                const Icon = template.icon;
                return (
                  <button
                    key={template.id}
                    onClick={() => handleSelectTemplate(template)}
                    className="flex items-start gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent"
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-muted">
                      <Icon className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="font-medium">{template.name}</div>
                      <div className="mt-1 text-xs text-muted-foreground line-clamp-2">
                        {template.description}
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{selectedTemplate.name}</DialogTitle>
              <DialogDescription>
                Customize the agent name before creating
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              <div>
                <Label className="text-xs text-muted-foreground">Agent Name</Label>
                <Input
                  value={customName}
                  onChange={(e) => setCustomName(e.target.value)}
                  className="mt-1"
                  placeholder="Enter agent name"
                />
              </div>

              <div>
                <Label className="text-xs text-muted-foreground">Instructions Preview</Label>
                <div className="mt-1 rounded-md border bg-muted/50 p-3 text-xs font-mono whitespace-pre-wrap max-h-40 overflow-y-auto">
                  {selectedTemplate.instructions.slice(0, 300)}...
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                {selectedTemplate.suggested_tools.map((tool) => (
                  <span key={tool} className="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">
                    {tool}
                  </span>
                ))}
              </div>
            </div>

            <DialogFooter>
              <Button variant="ghost" onClick={() => setSelectedTemplate(null)}>
                Back
              </Button>
              <Button onClick={handleCreate} disabled={creating || !customName.trim()}>
                {creating ? "Creating..." : "Create Agent"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}