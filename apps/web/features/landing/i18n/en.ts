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
