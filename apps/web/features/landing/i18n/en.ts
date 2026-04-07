import type { LandingDict } from "./types";

export const en: LandingDict = {
  header: {
    login: "Log in",
    dashboard: "Dashboard",
  },

  theater: {
    kicker: "Agent Runtime Theater",
    headlineLine1: "See one task move",
    headlineLine2: "from intent to merged work.",
    description:
      "Agentra keeps the full execution loop visible. Work lands once, the right runtime picks it up, skills shape the run, and the outcome compounds into the next task.",
    primaryCta: "Open workspace",
    secondaryCta: "View GitHub",
    worksWith: "Runs with",
    stepLabel: "Workflow loop",
    liveLabel: "Live state",
    cycleLabel: "Auto cycle",
    cycleHint: "The sequence keeps looping, so each completed step feeds the next run.",
    sceneAriaLabel:
      "Animated workflow board showing how Agentra moves work through five stages",
    proofChips: [
      "One surface for humans and agents",
      "Local and cloud runtimes",
      "Reusable skills compound every run",
    ],
    steps: [
      {
        id: "capture",
        label: "Capture",
        title: "Capture real work",
        description:
          "Start from an issue, request, or production signal. Context, ownership, and acceptance criteria arrive before the agent run starts.",
        statusLabel: "Status",
        statusValue: "Issue triaged",
        resultLabel: "Payload",
        resultValue: "Brief, repo, acceptance criteria",
        meta: "Fresh work lands in a shared queue instead of a private prompt tab.",
      },
      {
        id: "assign",
        label: "Assign",
        title: "Route it to the right runtime",
        description:
          "Choose the agent, provider, and workspace runtime that fits the task. The same surface handles humans, local daemons, and cloud runners.",
        statusLabel: "Status",
        statusValue: "Runtime matched",
        resultLabel: "Payload",
        resultValue: "Agent, provider, permissions",
        meta: "Execution starts with the right machine and the right teammate identity.",
      },
      {
        id: "execute",
        label: "Execute",
        title: "Run with streamed execution",
        description:
          "The task is claimed, tools fire, and progress stays visible. You can watch the run, inspect blockers, and keep the timeline current without polling.",
        statusLabel: "Status",
        statusValue: "Streaming live",
        resultLabel: "Payload",
        resultValue: "Logs, tool calls, blocker signals",
        meta: "Every transition is visible while the runtime stays online.",
      },
      {
        id: "review",
        label: "Review",
        title: "Review the outcome in context",
        description:
          "Outputs return to the workspace as comments, status changes, and follow-up work. Humans review inside the same operational thread.",
        statusLabel: "Status",
        statusValue: "Ready for review",
        resultLabel: "Payload",
        resultValue: "Diff, summary, follow-ups",
        meta: "Review happens where the task lives, not in a disconnected terminal log.",
      },
      {
        id: "compound",
        label: "Compound",
        title: "Turn wins into reusable capability",
        description:
          "Successful runs become repeatable skills and trusted runtime patterns. Each completed task upgrades how the next one executes.",
        statusLabel: "Status",
        statusValue: "Skill captured",
        resultLabel: "Payload",
        resultValue: "Reusable workflow, durable memory",
        meta: "The system gets sharper with use instead of resetting every session.",
      },
    ],
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
      "The open control plane for software teams running coding agents.",
    cta: "Open workspace",
    links: {
      about: "About",
      changelog: "Changelog",
      github: "GitHub",
    },
    copyright: "\u00a9 {year} Agentra. All rights reserved.",
  },

  about: {
    title: "About Agentra",
    paragraphs: [
      "Agentra is an open-source task management platform built for software teams working with coding agents.",
      "It gives agents a real operating surface: assign issues, observe progress, manage runtimes, and turn repeatable workflows into reusable skills.",
      "You can self-host it, inspect the full stack, and adapt it to your own infrastructure and agent setup.",
    ],
    cta: "View on GitHub",
  },

  changelog: {
    title: "Changelog",
    subtitle: "New updates and improvements to Agentra.",
    entries: [
      {
        version: "0.1.7",
        date: "2026-04-07",
        title: "Local-First Runtime & Landing Refresh",
        changes: [
          "CLI and daemon flows now target local Agentra services and OrbStack domains by default instead of stale remote endpoints",
          "Runtime network endpoints moved behind environment variables for easier local deployment and overrides",
          "Runtime cards in the web app now report the installed local CLI version instead of a generic dev label",
          "Docker Compose now uses the internal postgres service for server and migration containers, so the self-hosted stack boots reliably",
          "Landing page redesigned into a single execution theater that explains capture, assign, execute, review, and compound in one view",
          "Homepage visualization now falls back gracefully when WebGL is unavailable, preventing client-side crashes on unsupported environments",
        ],
      },
    ],
  },
};
