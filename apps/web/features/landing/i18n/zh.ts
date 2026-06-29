import type { LandingDict } from "./types";

export const zh: LandingDict = {
  header: {
    login: "\u767b\u5f55",
    dashboard: "\u8fdb\u5165\u5de5\u4f5c\u53f0",
  },

  theater: {
    kicker: "AI \u5de5\u4f5c\u63a7\u5236\u53f0",
    headlineLine1: "\u8ba9 coding agents",
    headlineLine2: "\u50cf\u961f\u53cb\u4e00\u6837\u5de5\u4f5c\u3002",
    description:
      "Agentra \u8ba9\u4efb\u52a1\u53ea\u9700\u5206\u914d\u4e00\u6b21\uff1a\u4ea4\u63a5\u3001\u6267\u884c\u3001review \u548c skill \u6c89\u6dc0\u90fd\u7559\u5728\u540c\u4e00\u4e2a\u5de5\u4f5c\u9762\u3002\u5b83\u628a\u5355\u6b21 agent run \u53d8\u6210\u56e2\u961f\u53ef\u7ba1\u7406\u7684\u6267\u884c\u6d41\u7a0b\u3002",
    primaryCta: "\u8fdb\u5165\u5de5\u4f5c\u53f0",
    secondaryCta: "\u67e5\u770b GitHub",
    worksWith: "\u8fd0\u884c\u4e8e",
    stepLabel: "\u5171\u4eab\u6d41\u7a0b",
    liveLabel: "\u4ea7\u54c1\u8bc1\u636e",
    sceneAriaLabel:
      "Agentra \u4ea7\u54c1\u8bc1\u636e\u9762\u677f\uff0c\u5c55\u793a AI \u4efb\u52a1\u5728\u5171\u4eab\u5de5\u4f5c\u6d41\u4e2d\u7684\u52a8\u6001\u8fc1\u79fb",
    proofChips: [
      "\u4efb\u52a1\u4e0d\u518d\u843d\u5728\u79c1\u6709 prompt \u91cc",
      "review \u5c31\u53d1\u751f\u5728 run \u7684\u4e0a\u4e0b\u6587\u91cc",
      "\u6210\u529f run \u4f1a\u6c89\u6dc0\u4e3a\u56e2\u961f\u53ef\u590d\u7528 skill",
    ],
    panelTaskLabel: "\u5f53\u524d\u4efb\u52a1",
    panelTaskValue:
      "\u4fee\u590d workspace invite OAuth \u56de\u5f52\uff0c\u540c\u65f6\u4e0d\u7834\u574f SSO \u6d41\u7a0b\u3002",
    panelQueueLabel: "\u4efb\u52a1\u5165\u53e3",
    panelQueueValue: "Issue #1842 \u00b7 shared workspace queue",
    panelRuntimeLabel: "Runtime",
    panelReviewLabel: "\u4eba\u5de5\u68c0\u67e5\u70b9",
    panelArtifactLabel: "\u56e2\u961f\u4f1a\u7559\u4e0b\u4ec0\u4e48",
    panelOwnerLabel: "\u5f53\u524d\u4ea4\u63a5",
    panelNextLabel: "\u4e0b\u4e00\u4e2a\u7ed3\u679c",
    taskPacketLabel: "\u4efb\u52a1\u5305",
    activeFocusLabel: "\u5f53\u524d\u7126\u70b9",
    stageNoteLabel: "\u9636\u6bb5\u8bf4\u660e",
    steps: [
      {
        id: "capture",
        label: "\u4efb\u52a1",
        title: "\u4efb\u52a1\u5148\u8fdb\u5165\u56e2\u961f\u5171\u4eab\u4e0a\u4e0b\u6587",
        description:
          "\u5de5\u4f5c\u4ece\u771f\u5b9e issue \u548c\u9a8c\u6536\u6761\u4ef6\u5f00\u59cb\uff0c\u800c\u4e0d\u662f\u4ece\u4e00\u6bb5\u4e34\u65f6 prompt \u5f00\u59cb\u3002",
        statusLabel: "\u72b6\u6001",
        statusValue: "\u4efb\u52a1\u5305\u5df2\u9501\u5b9a",
        resultLabel: "\u8f93\u5165",
        resultValue: "\u4efb\u52a1\u7b80\u62a5\u3001\u4ed3\u5e93\u3001\u9a8c\u6536\u6761\u4ef6",
        meta: "\u8fd9\u91cc\u5b58\u7684\u662f\u771f\u5b9e\u5de5\u4f5c\uff0c\u4e0d\u662f\u4e34\u65f6\u62fc\u51fa\u6765\u7684 prompt\u3002",
        signal: "\u8f93\u5165\u5b8c\u6574",
        owner: "PM + Eng lead",
        artifact: "\u4efb\u52a1\u7b80\u62a5\u5df2\u5f52\u6863\uff0crepo \u4e0a\u4e0b\u6587\u5df2\u6302\u8f7d",
        review:
          "\u4eba\u53ea\u9700\u5b9a\u4e49\u4e00\u6b21\u76ee\u6807\u548c\u98ce\u9669\u8fb9\u754c\uff0c\u4e0d\u7528\u6bcf\u6b21\u91cd\u8bb2\u80cc\u666f\u3002",
        nextAction: "\u5e26\u7740\u5b8c\u6574 task packet \u5f00\u59cb agent \u4ea4\u63a5",
      },
      {
        id: "assign",
        label: "\u8def\u7531",
        title: "\u4efb\u52a1\u88ab\u8def\u7531\u5230\u5408\u9002\u7684 agent",
        description:
          "Agentra \u4f1a\u4e3a\u8fd9\u4e2a\u5de5\u4f5c\u5339\u914d\u5408\u9002\u7684 provider\u3001\u673a\u5668\u548c workspace \u8eab\u4efd\u3002",
        statusLabel: "\u72b6\u6001",
        statusValue: "\u5df2\u5339\u914d Codex runtime",
        resultLabel: "\u8f93\u5165",
        resultValue: "Agent\u3001provider\u3001\u6743\u9650\u4e0a\u4e0b\u6587",
        meta:
          "\u56e2\u961f\u53ef\u4ee5\u770b\u5230\u662f\u8c01\u63a5\u624b\u4e86\u4efb\u52a1\uff0c\u4e3a\u4ec0\u4e48\u662f\u8fd9\u4e2a runtime\u3002",
        signal: "runtime \u5c31\u7eea",
        owner: "Runtime router",
        artifact: "\u5df2\u5206\u914d Codex \u00b7 macOS daemon\uff0c\u5e26 workspace \u6743\u9650",
        review:
          "\u4ea4\u63a5\u8fc7\u7a0b\u662f\u53ef\u89c1\u7684\uff0c\u6240\u4ee5\u5b83\u770b\u8d77\u6765\u662f\u8fd0\u8425\u6d41\u7a0b\uff0c\u800c\u4e0d\u662f\u9ed1\u7bb1\u9b54\u6cd5\u3002",
        nextAction: "\u7528\u6b63\u786e\u7684\u673a\u5668\u548c\u6743\u9650\u5f00\u59cb run",
      },
      {
        id: "execute",
        label: "\u8fd0\u884c",
        title: "\u8fd0\u884c\u8fc7\u7a0b\u5bf9\u56e2\u961f\u6301\u7eed\u53ef\u89c1",
        description:
          "\u56e2\u961f\u4e0d\u7528\u8ffd\u7ec8\u7aef tab\uff0c\u4e5f\u80fd\u770b\u5230\u8fdb\u5ea6\u3001blocker \u548c\u8f93\u51fa\u3002",
        statusLabel: "\u72b6\u6001",
        statusValue: "\u6b63\u5728\u6d41\u5f0f\u6267\u884c",
        resultLabel: "\u8f93\u51fa",
        resultValue: "\u65e5\u5fd7\u3001tool calls\u3001blocker \u4fe1\u53f7",
        meta:
          "\u6267\u884c\u7559\u5728\u5171\u4eab\u7ebf\u7d22\u91cc\uff0c\u4e0d\u4f1a\u6d88\u5931\u5728\u67d0\u4e2a\u79c1\u6709 session \u91cc\u3002",
        signal: "logs live",
        owner: "Codex agent",
        artifact: "\u6d4b\u8bd5\u3001\u4fee\u6539\u548c\u5931\u8d25\u70b9\u5df2\u540c\u6b65\u56de\u4efb\u52a1\u6d41",
        review:
          "\u8bc4\u5ba1\u4eba\u4e0d\u5fc5\u7b49 run \u7ed3\u675f\u624d\u80fd\u770b\uff0c\u56e0\u4e3a\u8fd9\u6761\u8f68\u8ff9\u81ea\u5e26\u53ef\u68c0\u67e5\u7684\u8bc1\u636e\u3002",
        nextAction: "\u7ed3\u679c\u76f4\u63a5\u6389\u5165 review\uff0c\u4e0d\u4e22\u4e0a\u4e0b\u6587",
      },
      {
        id: "review",
        label: "\u8bc4\u5ba1",
        title: "\u4eba\u5728\u540c\u4e00\u6761\u7ebf\u7d22\u5185\u5b8c\u6210 review",
        description:
          "diff\u3001\u603b\u7ed3\u548c follow-up \u90fd\u76f4\u63a5\u56de\u5230\u539f\u4efb\u52a1\u4e0a\uff0c\u4e0d\u9700\u8981\u53e6\u5f00\u7cfb\u7edf\u3002",
        statusLabel: "\u72b6\u6001",
        statusValue: "\u5f85\u4eba\u5de5\u8bc4\u5ba1",
        resultLabel: "\u8f93\u51fa",
        resultValue: "\u4ee3\u7801 diff\u3001\u603b\u7ed3\u3001\u540e\u7eed\u4efb\u52a1",
        meta:
          "review \u4e0d\u662f\u53e6\u4e00\u5957\u7cfb\u7edf\uff0c\u5b83\u5c31\u662f\u8fd9\u6761\u6d41\u7a0b\u7684\u4e00\u90e8\u5206\u3002",
        signal: "review pending",
        owner: "Reviewer",
        artifact: "\u53d8\u66f4\u6458\u8981\u3001\u98ce\u9669\u8bf4\u660e\u548c follow-up \u5df2\u751f\u6210",
        review:
          "reviewer \u770b\u5230 task\u3001run \u548c outcome \u540c\u5c4f\uff0c\u6240\u4ee5\u51b3\u7b56\u66f4\u5feb\uff0c\u98ce\u9669\u66f4\u53ef\u63a7\u3002",
        nextAction: "\u8fd9\u5957\u5df2\u8fc7\u5ba1\u7684\u505a\u6cd5\u53ef\u4ee5\u88ab\u4fdd\u5b58\u590d\u7528",
      },
      {
        id: "compound",
        label: "\u590d\u7528",
        title: "\u8fd9\u6b21\u80dc\u5229\u65b9\u5f0f\u53ef\u4ee5\u88ab\u590d\u7528",
        description:
          "\u4e00\u6b21\u6210\u529f run \u53ef\u4ee5\u5b58\u4e3a reusable skill\uff0c\u8ba9\u4e0b\u4e00\u6b21\u4efb\u52a1\u76f4\u63a5\u4ece\u66f4\u9ad8\u57fa\u7ebf\u5f00\u59cb\u3002",
        statusLabel: "\u72b6\u6001",
        statusValue: "\u5df2\u6c89\u6dc0\u4e3a skill",
        resultLabel: "\u8f93\u51fa",
        resultValue: "workflow\u3001team memory\u3001runtime \u504f\u597d",
        meta:
          "\u56e2\u961f\u7559\u4e0b\u7684\u662f\u65b9\u6cd5\uff0c\u4e0d\u53ea\u662f\u67d0\u6b21 run \u7684\u7b54\u6848\u3002",
        signal: "\u57fa\u7ebf\u6298\u9ad8",
        owner: "Shared team memory",
        artifact: "\u5df2\u9a8c\u8bc1\u7684 workflow \u88ab\u4fdd\u5b58\u4e3a reusable skill",
        review:
          "\u56e2\u961f\u4e0d\u7528\u518d\u91cd\u642d\u4e00\u5957 prompt choreography\uff0c\u76f4\u63a5\u590d\u7528\u5df2\u8bc1\u660e\u6709\u6548\u7684\u505a\u6cd5\u3002",
        nextAction: "\u4e0b\u4e00\u4e2a\u4efb\u52a1\u76f4\u63a5\u7ee7\u627f\u8fd9\u4e2a\u66f4\u9ad8\u57fa\u7ebf",
      },
    ],
  },

  valueProps: {
    label: "为什么需要 Agentra",
    headline: "Agent CLI 和真实团队执行之间，缺的就是这一层。",
    description:
      "编码 Agent 会执行，但团队还需要协同、可见性、可复用能力和 runtime 控制。Agentra 就是这层管理系统。",
    items: [
      {
        title: "协同",
        description:
          "把 prompt 变成可跟踪的工作。每个任务都有 owner、生命周期，以及人和 Agent 之间清晰的交接。",
      },
      {
        title: "可见性",
        description:
          "不用切换五个工具，也能知道谁在工作、改了什么、卡在哪里，以及哪些 runtime 正常。",
      },
      {
        title: "复利",
        description:
          "把好的工作流封装成技能，让每一次成功的执行都在增强整个团队，而不只是一次 Agent session。",
      },
    ],
  },

  footer: {
    tagline:
      "\u9762\u5411\u7f16\u7801 Agent \u56e2\u961f\u7684\u5f00\u6e90\u63a7\u5236\u5c42\u3002",
    cta: "\u8fdb\u5165\u5de5\u4f5c\u53f0",
    links: {
      about: "\u5173\u4e8e",
      changelog: "\u66f4\u65b0\u65e5\u5fd7",
      github: "GitHub",
    },
    copyright: "\u00a9 {year} Agentra. \u4fdd\u7559\u6240\u6709\u6743\u5229\u3002",
  },

  about: {
    title: "\u5173\u4e8e Agentra",
    paragraphs: [
      "Agentra \u662f\u4e00\u4e2a\u9762\u5411\u4f7f\u7528\u7f16\u7801 Agent \u7684\u8f6f\u4ef6\u56e2\u961f\u7684\u5f00\u6e90\u4efb\u52a1\u7ba1\u7406\u5e73\u53f0\u3002",
      "\u5b83\u4e3a Agent \u63d0\u4f9b\u771f\u6b63\u7684\u8fd0\u4f5c\u754c\u9762\uff1a\u5206\u914d Issue\u3001\u89c2\u5bdf\u8fdb\u5ea6\u3001\u7ba1\u7406 runtime\uff0c\u5e76\u628a\u53ef\u590d\u7528\u7684\u5de5\u4f5c\u6d41\u6c89\u6dc0\u4e3a\u6280\u80fd\u3002",
      "\u4f60\u53ef\u4ee5\u81ea\u6258\u7ba1\uff0c\u5ba1\u67e5\u6574\u4e2a\u6808\uff0c\u5e76\u6309\u7167\u81ea\u5df1\u7684\u57fa\u7840\u8bbe\u65bd\u548c Agent \u65b9\u5f0f\u8fdb\u884c\u5b9a\u5236\u3002",
    ],
    cta: "\u5728 GitHub \u4e0a\u67e5\u770b",
  },

  changelog: {
    title: "\u66f4\u65b0\u65e5\u5fd7",
    subtitle: "Agentra \u7684\u6700\u65b0\u66f4\u65b0\u548c\u6539\u8fdb\u3002",
    entries: [
      {
        version: "0.0.2",
        date: "2026-04-07",
        title: "\u672c\u5730\u9ed8\u8ba4\u914d\u7f6e\u4e0e Docker \u4fee\u590d",
        changes: [
          "CLI \u767b\u5f55\u3001API \u548c daemon \u7684\u9ed8\u8ba4\u5730\u5740\u5df2\u5207\u6362\u4e3a\u672c\u5730 Agentra \u670d\u52a1\u548c OrbStack \u57df\u540d",
          "\u4e0d\u518d\u4f18\u5148\u4f7f\u7528\u8fc7\u65f6\u7684\u8fdc\u7a0b\u914d\u7f6e\uff0c\u672c\u5730\u9ed8\u8ba4\u503c\u4f1a\u4f18\u5148\u751f\u6548",
          "\u79fb\u9664\u4e86 install \u548c update \u6d41\u7a0b\u4e2d\u5df2\u635f\u574f\u7684 Homebrew tap \u5f15\u7528",
          "\u6536\u7d27 Docker \u4e0e compose \u9ed8\u8ba4\u503c\uff0c\u8ba9\u672c\u5730\u81ea\u6258\u7ba1\u6808\u66f4\u5bb9\u6613\u542f\u52a8",
          "\u516c\u5f00\u6587\u6863\u548c\u8df3\u8f6c\u8def\u5f84\u5df2\u6309\u7167\u5f53\u524d\u672c\u5730\u4f18\u5148\u65b9\u5f0f\u7b80\u5316",
        ],
      },
      {
        version: "0.0.1",
        date: "2026-04-04",
        title: "Agentra \u9996\u4e2a\u516c\u5f00\u843d\u5730\u9875\u7248\u672c",
        changes: [
          "\u53d1\u5e03 Agentra \u65b0\u54c1\u724c\u548c\u65b0\u4ea7\u54c1\u53d9\u4e8b\u7684\u9996\u7248\u9996\u9875",
          "\u91cd\u505a landing hero \u548c\u5404\u652f\u6491\u677f\u5757\uff0c\u4ece repo \u5bfc\u5411\u8f6c\u4e3a\u4ea7\u54c1\u5bfc\u5411",
          "\u4e3a\u516c\u5f00\u7ad9\u70b9\u65b0\u589e\u4e86\u843d\u5730\u9875\u89c6\u89c9\u8d44\u4ea7\u548c\u80cc\u666f\u7d20\u6750",
        ],
      },
    ],
  },
};
