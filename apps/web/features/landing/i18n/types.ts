export type Locale = "en" | "zh";

export const locales: Locale[] = ["en", "zh"];

export const localeLabels: Record<Locale, string> = {
  en: "EN",
  zh: "\u4e2d\u6587",
};

type FeatureSection = {
  label: string;
  title: string;
  description: string;
  cards: { title: string; description: string }[];
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
    cycleLabel: string;
    cycleHint: string;
    sceneAriaLabel: string;
    proofChips: string[];
    panelTaskLabel: string;
    panelTaskValue: string;
    panelQueueLabel: string;
    panelQueueValue: string;
    panelRuntimeLabel: string;
    panelFeedLabel: string;
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
  hero: {
    kicker: string;
    headlineLine1: string;
    headlineLine2: string;
    subheading: string;
    cta: string;
    worksWith: string;
    proofChips: string[];
    imageAlt: string;
  };
  valueProps: {
    label: string;
    headline: string;
    description: string;
    items: { title: string; description: string }[];
  };
  features: {
    teammates: FeatureSection;
    autonomous: FeatureSection;
    skills: FeatureSection;
    runtimes: FeatureSection;
  };
  howItWorks: {
    label: string;
    headlineMain: string;
    headlineFaded: string;
    steps: { title: string; description: string }[];
    cta: string;
  };
  openSource: {
    label: string;
    headlineLine1: string;
    headlineLine2: string;
    description: string;
    cta: string;
    highlights: { title: string; description: string }[];
  };
  faq: {
    label: string;
    headline: string;
    items: { question: string; answer: string }[];
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
