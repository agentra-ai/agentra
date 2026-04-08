import type { LandingDict } from "./types";

export const en: LandingDict = {
  header: {
    login: "Log in",
    dashboard: "Dashboard",
  },

  theater: {
    kicker: "AI Work Control Plane",
    headlineLine1: "Make coding agents",
    headlineLine2: "work like teammates.",
    description:
      "Assign a task once, watch the handoff, review in context, and keep the winning workflow as a reusable skill. Agentra turns isolated agent runs into shared team execution.",
    primaryCta: "Open workspace",
    secondaryCta: "View GitHub",
    worksWith: "Runs with",
    stepLabel: "Shared flow",
    liveLabel: "Product proof",
    cycleLabel: "One task. One shared thread.",
    cycleHint:
      "A real task moves through one shared thread: assigned once, executed visibly, reviewed in context, then saved as reusable team capability.",
    sceneAriaLabel:
      "Agentra product panel showing a shared AI task moving through the team workflow",
    proofChips: [
      "Tasks stop living in private prompts",
      "Review happens in the same context as the run",
      "Successful runs become reusable team skills",
    ],
    panelTaskLabel: "Current task",
    panelTaskValue: "Ship the workspace invite OAuth fix without breaking SSO.",
    panelQueueLabel: "Task source",
    panelQueueValue: "Issue #1842 · shared workspace queue",
    panelRuntimeLabel: "Runtime",
    panelFeedLabel: "Shared flow",
    panelReviewLabel: "Human checkpoint",
    panelArtifactLabel: "What the team keeps",
    panelOwnerLabel: "Current handoff",
    panelNextLabel: "Next outcome",
    taskPacketLabel: "Task packet",
    activeFocusLabel: "Active focus",
    stageNoteLabel: "Stage note",
    steps: [
      {
        id: "capture",
        label: "Task",
        title: "A task enters shared team context",
        description:
          "The work starts as a real issue with context and acceptance criteria already attached.",
        statusLabel: "Status",
        statusValue: "Task packet locked",
        resultLabel: "Payload",
        resultValue: "Brief, repo, acceptance criteria",
        meta: "The run starts from real work, not from someone improvising a new prompt.",
        signal: "Input complete",
        owner: "PM + Eng lead",
        artifact: "Brief attached and repo context mounted",
        review:
          "Humans define the goal once instead of re-explaining background every time an agent runs.",
        nextAction: "Agent handoff starts with the full task packet",
      },
      {
        id: "assign",
        label: "Route",
        title: "The task is routed to the right agent",
        description:
          "Agentra chooses the provider, machine, and workspace identity that fit the work.",
        statusLabel: "Status",
        statusValue: "Codex runtime matched",
        resultLabel: "Payload",
        resultValue: "Agent, provider, permissions",
        meta: "The team can see which runtime picked up the task and why it was chosen.",
        signal: "Runtime ready",
        owner: "Runtime router",
        artifact: "Codex on macOS daemon with workspace access",
        review:
          "Routing is visible, so the handoff feels operational instead of magical.",
        nextAction: "The run begins with the correct machine and permissions",
      },
      {
        id: "execute",
        label: "Run",
        title: "The run stays visible while it works",
        description:
          "The team can watch progress, blockers, and output without chasing a terminal tab.",
        statusLabel: "Status",
        statusValue: "Streaming execution",
        resultLabel: "Payload",
        resultValue: "Logs, tool calls, blocker signals",
        meta: "Execution stays on the shared thread instead of disappearing into a private session.",
        signal: "Logs live",
        owner: "Codex agent",
        artifact: "Tests, edits, and failures synced back to the task",
        review:
          "Reviewers can step in early because the run leaves behind an inspectable trail.",
        nextAction: "The finished run drops into review without losing context",
      },
      {
        id: "review",
        label: "Review",
        title: "Humans review inside the same thread",
        description:
          "Diffs, summaries, and follow-up work land back where the task already lives.",
        statusLabel: "Status",
        statusValue: "Ready for review",
        resultLabel: "Payload",
        resultValue: "Diff, summary, follow-ups",
        meta: "Review is part of the same workflow, not a second system layered on top.",
        signal: "Review pending",
        owner: "Reviewer",
        artifact: "Diff summary, risk notes, and follow-up tasks generated",
        review:
          "The reviewer sees the task, the run, and the outcome together, so approval is faster and safer.",
        nextAction: "The approved pattern is ready to be saved for reuse",
      },
      {
        id: "compound",
        label: "Reuse",
        title: "The winning pattern becomes reusable",
        description:
          "A successful run can be saved as a reusable skill so the next task starts higher than the last one.",
        statusLabel: "Status",
        statusValue: "Skill captured",
        resultLabel: "Payload",
        resultValue: "Workflow, durable memory, runtime preference",
        meta: "The team keeps the method, not just the answer from one good run.",
        signal: "Baseline up",
        owner: "Shared team memory",
        artifact: "Validated workflow saved as a reusable skill",
        review:
          "Instead of repeating prompt choreography, the team reuses a proven operating pattern.",
        nextAction: "The next task inherits the upgraded baseline",
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
        version: "0.0.2",
        date: "2026-04-07",
        title: "Local Defaults & Docker Fixes",
        changes: [
          "CLI login, API, and daemon defaults now target local Agentra services and OrbStack domains",
          "Stale remote configuration is no longer preferred over local service defaults",
          "Broken Homebrew tap references were removed from install and update flows",
          "Docker and compose defaults were tightened so local self-hosting boots more reliably",
          "Public docs and redirects were simplified for the current local-first setup",
        ],
      },
      {
        version: "0.0.1",
        date: "2026-04-04",
        title: "Initial Agentra Landing Release",
        changes: [
          "Introduced the Agentra homepage refresh with updated product narrative and branding",
          "Reworked the landing hero and supporting sections to focus on the product instead of repo-first messaging",
          "Added new landing visuals and supporting backgrounds for the public site",
        ],
      },
    ],
  },
};
