export type Locale = "en" | "zh";

export const locales: Locale[] = ["en", "zh"];

export const localeLabels: Record<Locale, string> = {
  en: "EN",
  zh: "\u4e2d\u6587",
};

export type LandingDict = {
  header: { login: string; dashboard: string };
  theater: {
    kicker: string;
    headlineLine1: string;
    headlineLine2: string;
    description: string;
    primaryCta: string;
    secondaryCta: string;
    worksWith: string;
    stepLabel: string;
    liveLabel: string;
    sceneAriaLabel: string;
    proofChips: string[];
    panelTaskLabel: string;
    panelTaskValue: string;
    panelQueueLabel: string;
    panelQueueValue: string;
    panelRuntimeLabel: string;
    panelReviewLabel: string;
    panelArtifactLabel: string;
    panelOwnerLabel: string;
    panelNextLabel: string;
    taskPacketLabel: string;
    activeFocusLabel: string;
    stageNoteLabel: string;
    steps: {
      id: string;
      label: string;
      title: string;
      description: string;
      statusLabel: string;
      statusValue: string;
      resultLabel: string;
      resultValue: string;
      meta: string;
      signal: string;
      owner: string;
      artifact: string;
      review: string;
      nextAction: string;
    }[];
  };
  valueProps: {
    label: string;
    headline: string;
    description: string;
    items: { title: string; description: string }[];
  };
  footer: {
    tagline: string;
    cta: string;
    links: {
      about: string;
      changelog: string;
      github: string;
    };
    copyright: string;
  };
  about: {
    title: string;
    paragraphs: string[];
    cta: string;
  };
  changelog: {
    title: string;
    subtitle: string;
    entries: {
      version: string;
      date: string;
      title: string;
      changes: string[];
    }[];
  };
};

