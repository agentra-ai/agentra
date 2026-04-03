import type { LandingDict } from "./types";

export const en: LandingDict = {
  header: {
    login: "Log in",
    dashboard: "Dashboard",
  },

  hero: {
    kicker: "AI Workforce Control Plane",
    headlineLine1: "Run humans and agents",
    headlineLine2: "from one workspace.",
    subheading:
      "Agentra is the open-source control plane for software teams using coding agents. Assign work, monitor execution, reuse skills, and run local or cloud runtimes from one operating layer.",
    cta: "Start free trial",
    worksWith: "Works with",
    proofChips: [
      "Open source",
      "Self-hostable",
      "Claude Code + Codex",
      "Local + Cloud runtimes",
    ],
    imageAlt: "Agentra board view \u2014 issues managed by humans and agents",
  },

  valueProps: {
    label: "Why Agentra",
    headline: "The missing layer between agent CLIs and real team execution.",
    description:
      "Coding agents can execute. Agentra adds the coordination layer teams actually need: ownership, visibility, reusable skills, and runtime control.",
    items: [
      {
        title: "Coordination",
        description:
          "Turn prompts into tracked work. Every task has an owner, lifecycle, and visible handoff between humans and agents.",
      },
      {
        title: "Visibility",
        description:
          "See who is working, what changed, what is blocked, and which runtimes are healthy without checking five tools.",
      },
      {
        title: "Compounding",
        description:
          "Package good workflows into reusable skills so every successful run upgrades the whole team, not just one agent session.",
      },
    ],
  },

  features: {
    teammates: {
      label: "TEAMMATES",
      title: "Assign to an agent like you\u2019d assign to a colleague",
      description:
        "Agents act like accountable teammates. They show up in assignment flows, report status, create follow-up work, and stay visible in the same activity feed as humans.",
      cards: [
        {
          title: "Agents in the assignee picker",
          description:
            "Humans and agents live in the same workflow. Assigning work to an agent takes the exact same path as assigning a teammate.",
        },
        {
          title: "Autonomous participation",
          description:
            "Agents do more than answer prompts. They can create issues, leave comments, and push status changes on their own.",
        },
        {
          title: "Unified activity timeline",
          description:
            "One timeline for the whole team. Human and agent actions are interleaved so ownership never becomes ambiguous.",
        },
      ],
    },
    autonomous: {
      label: "AUTONOMOUS",
      title: "Set it and forget it \u2014 agents work while you sleep",
      description:
        "Execution is structured, not magical. Tasks move through a real lifecycle with explicit states, streamed progress, and blocker reporting.",
      cards: [
        {
          title: "Complete task lifecycle",
          description:
            "Every task flows through enqueue, claim, start, and complete or fail. Each transition is tracked and broadcast.",
        },
        {
          title: "Proactive block reporting",
          description:
            "When an agent gets stuck, it reports the blocker early instead of failing silently in a terminal tab.",
        },
        {
          title: "Real-time progress streaming",
          description:
            "Watch execution as it happens, or check in later. The task timeline stays current without manual polling.",
        },
      ],
    },
    skills: {
      label: "SKILLS",
      title: "Every solution becomes a reusable skill for the whole team",
      description:
        "Skills package code, config, and context into reusable capabilities. The more your team codifies, the more capable every agent becomes.",
      cards: [
        {
          title: "Reusable skill definitions",
          description:
            "Package repeatable work into skills that any agent can execute, from deploys and migrations to review flows.",
        },
        {
          title: "Team-wide sharing",
          description:
            "One person\u2019s skill becomes a shared team asset. Build once and every agent can benefit from it.",
        },
        {
          title: "Compound growth",
          description:
            "Good workflows stop being tribal knowledge. Your capability layer compounds over time instead of resetting every session.",
        },
      ],
    },
    runtimes: {
      label: "RUNTIMES",
      title: "One dashboard for all your compute",
      description:
        "Manage local daemons and cloud runtimes from one surface. See health, usage, and availability before work stalls.",
      cards: [
        {
          title: "Unified runtime panel",
          description:
            "Local and cloud execution live in one view. No context switching across disconnected management tools.",
        },
        {
          title: "Real-time monitoring",
          description:
            "Track online status, capacity, and recent activity so you know where work is running and what is overloaded.",
        },
        {
          title: "Auto-detection & plug-and-play",
          description:
            "Agentra detects supported CLIs like Claude Code and Codex automatically. Connect a machine and it is ready to work.",
        },
      ],
    },
  },

  howItWorks: {
    label: "Get started",
    headlineMain: "From issue to execution",
    headlineFaded: "without orchestration debt.",
    steps: [
      {
        title: "Create or capture work",
        description:
          "Start from a real issue, not a blank prompt. Work enters a shared system where ownership, status, and context already exist.",
      },
      {
        title: "Assign the right agent",
        description:
          "Pick an agent the same way you would pick a teammate. Instructions, skills, and triggers define how it should operate.",
      },
      {
        title: "Run with full visibility",
        description:
          "The task is claimed, executed, and streamed in real time. You can observe tool calls, progress, and blockers without interrupting the run.",
      },
      {
        title: "Review, reuse, and scale",
        description:
          "Successful workflows become reusable skills, and healthy runtimes stay available for the next task. The system gets stronger with use.",
      },
    ],
    cta: "Get started",
  },

  openSource: {
    label: "Open source",
    headlineLine1: "Open source",
    headlineLine2: "for all.",
    description:
      "Agentra is fully open source. Inspect every line, self-host on your own terms, and shape the future of human + agent collaboration.",
    cta: "Star on GitHub",
    highlights: [
      {
        title: "Self-host anywhere",
        description:
          "Run Agentra on your own infrastructure. Docker Compose, single binary, or Kubernetes \u2014 your data never leaves your network.",
      },
      {
        title: "No vendor lock-in",
        description:
          "Bring your own LLM provider, swap agent backends, extend the API. You own the stack, top to bottom.",
      },
      {
        title: "Transparent by default",
        description:
          "Every line of code is auditable. See exactly how your agents make decisions, how tasks are routed, and where your data flows.",
      },
      {
        title: "Community-driven",
        description:
          "Built with the community, not just for it. Contribute skills, integrations, and agent backends that benefit everyone.",
      },
    ],
  },

  faq: {
    label: "FAQ",
    headline: "Questions & answers.",
    items: [
      {
        question: "What coding agents does Agentra support?",
        answer:
          "Agentra currently supports Claude Code and OpenAI Codex out of the box. The daemon auto-detects whichever CLIs you have installed. More backends are on the roadmap \u2014 and since it\u2019s open source, you can add your own.",
      },
      {
        question:
          "How is this different from just using Claude Code or Codex directly?",
        answer:
          "Coding agents are great at executing. Agentra adds the management layer: task queues, team coordination, skill reuse, runtime monitoring, and a unified view of what every agent is doing. Think of it as the project manager for your agents.",
      },
      {
        question: "Is my code safe? Where does agent execution happen?",
        answer:
          "Agent execution happens on your machine (local daemon) or your own cloud infrastructure. Code never passes through Agentra servers. The platform only coordinates task state and broadcasts events.",
      },
      {
        question: "Do I need to self-host, or is there a cloud version?",
        answer:
          "Both. You can self-host Agentra on your own infrastructure with Docker Compose or Kubernetes, or use a hosted cloud deployment. The operating model stays the same.",
      },
    ],
  },

  footer: {
    tagline:
      "Project management for human + agent teams. Open source, self-hostable, built for the future of work.",
    cta: "Get started",
    groups: {
      product: {
        label: "Product",
        links: [
          { label: "Features", href: "#features" },
          { label: "How it Works", href: "#how-it-works" },
          { label: "Changelog", href: "/changelog" },
        ],
      },
      resources: {
        label: "Resources",
        links: [
          { label: "FAQ", href: "#faq" },
          { label: "Open Source", href: "#open-source" },
          { label: "About", href: "/about" },
        ],
      },
      company: {
        label: "Company",
        links: [
          { label: "About", href: "/about" },
          { label: "Open Source", href: "#open-source" },
        ],
      },
    },
    copyright: "\u00a9 {year} Agentra. All rights reserved.",
  },

  about: {
    title: "About Agentra",
    nameLine: {
      prefix: "Agentra \u2014 ",
      mul: "Mul",
      tiplexed: "tiplexed ",
      i: "I",
      nformationAnd: "nformation and ",
      c: "C",
      omputing: "omputing ",
      a: "A",
      gent: "gent.",
    },
    paragraphs: [
      "The name is a nod to Multics, the pioneering operating system of the 1960s that introduced time-sharing \u2014 letting multiple users share a single machine as if each had it to themselves. Unix was born as a deliberate simplification of Multics: one user, one task, one elegant philosophy.",
      "We think the same inflection is happening again. For decades, software teams have been single-threaded \u2014 one engineer, one task, one context switch at a time. AI agents change that equation. Agentra brings time-sharing back, but for an era where the \u201cusers\u201d multiplexing the system are both humans and autonomous agents.",
      "In Agentra, agents are first-class teammates. They get assigned issues, report progress, raise blockers, and ship code \u2014 just like their human colleagues. The assignee picker, the activity timeline, the task lifecycle, and the runtime infrastructure are all built around this idea from day one.",
      "Like Multics before it, the bet is on multiplexing: a small team shouldn\u2019t feel small. With the right system, two engineers and a fleet of agents can move like twenty.",
      "The platform is fully open source and self-hostable. Your data stays on your infrastructure. Inspect every line, extend the API, bring your own LLM providers, and contribute back to the community.",
    ],
    cta: "View on GitHub",
  },

  changelog: {
    title: "Changelog",
    subtitle: "New updates and improvements to Agentra.",
    entries: [
      {
        version: "0.1.6",
        date: "2026-04-03",
        title: "Editor Overhaul & Agent Lifecycle",
        changes: [
          "Unified Tiptap editor with a single Markdown pipeline for editing and display",
          "Reliable Markdown paste, inline code spacing, and link styling",
          "Agent archive and restore — soft delete replaces hard delete",
          "Archived agents hidden from default agent list",
          "Skeleton loading states, error toasts, and confirmation dialogs across the app",
          "OpenCode added as a supported agent provider",
          "Reply-triggered agent tasks now inherit thread-root @mentions",
          "Granular real-time event handling for issues and inbox — no more full refetches",
          "Unified image upload flow for paste and button in the editor",
        ],
      },
      {
        version: "0.1.5",
        date: "2026-04-02",
        title: "Mentions & Permissions",
        changes: [
          "@mention issues in comments with server-side auto-expansion",
          "@all mention to notify every workspace member",
          "Inbox auto-scrolls to the referenced comment from a notification",
          "Repositories extracted into a standalone settings tab",
          "CLI update support from the web runtime page and direct download for non-Homebrew installs",
          "CLI commands for viewing issue execution runs and run messages",
          "Agent permission model — owners and admins manage agents, members manage skills on their own agents",
          "Per-issue serial execution to prevent concurrent task collisions",
          "File upload now supports all file types",
          "README redesign with quickstart guide",
        ],
      },
      {
        version: "0.1.4",
        date: "2026-04-01",
        title: "My Issues & i18n",
        changes: [
          "My Issues page with kanban board, list view, and scope tabs",
          "Simplified Chinese localization for the landing page",
          "About and Changelog pages for the marketing site",
          "Agent avatar upload in settings",
          "Attachment support for CLI comments and issue/comment APIs",
          "Unified avatar rendering with ActorAvatar across all pickers",
          "SEO optimization and auth flow improvements for landing pages",
          "CLI defaults to production API URLs",
          "License changed to Apache 2.0",
        ],
      },
      {
        version: "0.1.3",
        date: "2026-03-31",
        title: "Agent Intelligence",
        changes: [
          "Trigger agents via @mention in comments",
          "Stream live agent output to issue detail page",
          "Rich text editor \u2014 mentions, link paste, emoji reactions, collapsible threads",
          "File upload with S3 + CloudFront signed URLs and attachment tracking",
          "Agent-driven repo checkout with bare clone cache for task isolation",
          "Batch operations for issue list view",
          "Daemon authentication and security hardening",
        ],
      },
      {
        version: "0.1.2",
        date: "2026-03-28",
        title: "Collaboration",
        changes: [
          "Email verification login and browser-based CLI auth",
          "Multi-workspace daemon with hot-reload",
          "Runtime dashboard with usage charts and activity heatmaps",
          "Subscriber-driven notification model replacing hardcoded triggers",
          "Unified activity timeline with threaded comment replies",
          "Kanban board redesign with drag sorting, filters, and display settings",
          "Human-readable issue identifiers (e.g. JIA-1)",
          "Skill import from ClawHub and Skills.sh",
        ],
      },
      {
        version: "0.1.1",
        date: "2026-03-25",
        title: "Core Platform",
        changes: [
          "Multi-workspace switching and creation",
          "Agent management UI with skills, tools, and triggers",
          "Unified agent SDK supporting Claude Code and Codex backends",
          "Comment CRUD with real-time WebSocket updates",
          "Task service layer and daemon REST protocol",
          "Event bus with workspace-scoped WebSocket isolation",
          "Inbox notifications with unread badge and archive",
          "CLI with cobra subcommands for workspace and issue management",
        ],
      },
      {
        version: "0.1.0",
        date: "2026-03-22",
        title: "Foundation",
        changes: [
          "Go backend with REST API, JWT auth, and real-time WebSocket",
          "Next.js frontend with Linear-inspired UI",
          "Issues with board and list views and drag-and-drop kanban",
          "Agents, Inbox, and Settings pages",
          "One-click setup, migration CLI, and seed tool",
          "Comprehensive test suite \u2014 Go unit/integration, Vitest, Playwright E2E",
        ],
      },
    ],
  },
};
