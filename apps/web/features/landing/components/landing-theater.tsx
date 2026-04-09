"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowRight,
  Bot,
  CheckCircle2,
  GitBranch,
  ShieldCheck,
  Sparkles,
  Workflow,
} from "lucide-react";
import { useAuthStore } from "@/features/auth";
import { cn } from "@/lib/utils";
import { useLocale } from "../i18n";
import {
  ClaudeCodeLogo,
  CodexLogo,
  headerButtonClassName,
} from "./shared";
import { LandingProofScene } from "./landing-proof-scene";

const LOOP_INTERVAL_MS = 4800;
const proofIcons = [Workflow, ShieldCheck, Sparkles];
const sceneNodePositions = [
  { left: "9%", top: "20%" },
  { left: "29%", top: "63%" },
  { left: "49%", top: "40%" },
  { left: "70%", top: "60%" },
  { left: "90%", top: "25%" },
];

export function LandingTheater() {
  const { t, locale } = useLocale();
  const user = useAuthStore((state) => state.user);
  const [activeIndex, setActiveIndex] = useState(0);
  const steps = t.theater.steps;
  const activeStep = steps[activeIndex] ?? steps[0];
  const fallbackSceneNodePosition = sceneNodePositions[0] ?? {
    left: "8%",
    top: "18%",
  };
  const headlineWidthClass =
    locale === "zh"
      ? "max-w-[13.2ch] sm:max-w-[13.8ch] lg:max-w-[14.4ch]"
      : "max-w-[11ch]";
  const headlineSecondaryFontClass =
    locale === "zh"
      ? "font-[family-name:var(--font-serif-zh)]"
      : "font-[family-name:var(--font-serif)]";
  const headlineSecondaryClass =
    locale === "zh"
      ? "text-[0.9em] leading-[1.02] tracking-[-0.06em] text-white/72"
      : "tracking-[-0.05em] text-white/70";
  const sceneProgress = `${((activeIndex + 1) / Math.max(steps.length, 1)) * 100}%`;
  const scenePacketTransform =
    activeIndex === 0
      ? "translate(-18%, -176%)"
      : activeIndex === steps.length - 1
        ? "translate(-82%, -146%)"
        : "translate(-50%, -146%)";

  useEffect(() => {
    if (steps.length === 0) {
      return;
    }

    const intervalId = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % steps.length);
    }, LOOP_INTERVAL_MS);

    return () => window.clearInterval(intervalId);
  }, [steps.length]);

  if (!activeStep) {
    return null;
  }

  const systemFacts = [
    {
      label: t.theater.panelOwnerLabel,
      value: activeStep.owner,
      icon: Bot,
    },
    {
      label: t.theater.panelReviewLabel,
      value: activeStep.review,
      icon: CheckCircle2,
    },
    {
      label: t.theater.panelArtifactLabel,
      value: activeStep.artifact,
      icon: GitBranch,
    },
  ];

  return (
    <main
      id="product"
      className="relative overflow-hidden bg-[#04070d] text-white"
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,_rgba(255,255,255,0.06),_transparent_30%),radial-gradient(circle_at_82%_18%,_rgba(17,24,39,0.62),_transparent_28%),radial-gradient(circle_at_76%_72%,_rgba(255,255,255,0.03),_transparent_18%),linear-gradient(180deg,_#05080d_0%,_#03060b_56%,_#020307_100%)]" />
      <div className="absolute inset-0 bg-[linear-gradient(rgba(148,163,184,0.032)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.032)_1px,transparent_1px)] [background-size:72px_72px] opacity-16" />
      <div className="absolute inset-x-0 top-0 h-px bg-white/12" />

      <section className="relative mx-auto max-w-[1360px] px-4 pb-14 pt-16 sm:px-6 sm:pb-16 sm:pt-20 lg:px-8 lg:pb-[4.5rem]">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,500px)_1fr] lg:items-start">
          <div className="relative z-10">
            <div className="inline-flex items-center gap-3 text-[11px] font-medium uppercase tracking-[0.2em] text-white/58">
              <span className="inline-flex h-px w-10 bg-[linear-gradient(90deg,rgba(255,255,255,0.42),rgba(255,255,255,0))]" />
              <Workflow className="size-3.5 text-white/62" />
              <span>{t.theater.kicker}</span>
            </div>

            <h1
              className={cn(
                "mt-5 text-[2.85rem] font-semibold leading-[0.88] tracking-[-0.085em] text-white sm:text-[3.75rem] lg:text-[4.35rem]",
                headlineWidthClass,
              )}
            >
              <span className="block">{t.theater.headlineLine1}</span>
              <span
                className={cn(
                  "mt-2 block",
                  headlineSecondaryFontClass,
                  headlineSecondaryClass,
                )}
              >
                {t.theater.headlineLine2}
              </span>
            </h1>

            <p className="mt-6 max-w-[35rem] text-[15px] leading-[1.95] text-white/62 sm:text-[16px]">
              {t.theater.description}
            </p>

            <div className="mt-7 flex flex-wrap items-center gap-3">
              <Link
                href={user ? "/issues" : "/login"}
                className={headerButtonClassName("solid", "dark")}
              >
                {user ? t.header.dashboard : t.theater.primaryCta}
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="https://github.com/agentra-ai/agentra"
                target="_blank"
                rel="noreferrer"
                className={headerButtonClassName("ghost", "dark")}
              >
                {t.theater.secondaryCta}
              </Link>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3 py-2 text-[12px] text-white/50 backdrop-blur-md">
                <span className="inline-flex size-1.5 rounded-full bg-white/68" />
                <span>{activeStep.signal}</span>
              </div>
            </div>

            <div className="mt-8 flex items-center gap-3 text-[11px] uppercase tracking-[0.18em] text-white/34">
              <span>{t.theater.stepLabel}</span>
              <span className="h-px flex-1 bg-white/8" />
            </div>

            <div className="mt-8 max-w-[34rem] divide-y divide-white/8 border-y border-white/8">
              {t.theater.proofChips.map((chip, index) => {
                const Icon = proofIcons[index] ?? Sparkles;

                return (
                  <div
                    key={chip}
                    className="grid grid-cols-[auto_auto_1fr] items-start gap-3 px-1 py-3.5 text-[13px] leading-[1.65] text-white/72"
                  >
                    <span className="inline-flex h-7 min-w-7 items-center justify-center rounded-full border border-white/10 bg-white/[0.03] px-1 text-[10px] font-semibold tabular-nums text-white/48">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                    <Icon className="mt-1 size-3.5 shrink-0 text-white/68" />
                    <span>{chip}</span>
                  </div>
                );
              })}
            </div>

            <div className="mt-5 flex flex-wrap items-center gap-3 text-[13px] text-white/48">
              <span>{t.theater.worksWith}</span>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-white/70">
                <ClaudeCodeLogo className="size-4 text-white/72" />
                <span>Claude Code</span>
              </div>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-white/70">
                <CodexLogo className="size-4 text-white/72" />
                <span>Codex</span>
              </div>
            </div>
          </div>

          <div className="relative">
            <div className="absolute -left-12 top-12 h-32 w-32 rounded-full bg-white/4 blur-3xl" />
            <div className="absolute -right-10 bottom-10 h-28 w-28 rounded-full bg-white/4 blur-3xl" />

            <div className="relative overflow-hidden rounded-[30px] bg-[linear-gradient(180deg,rgba(11,14,20,0.96),rgba(4,6,12,0.99))] shadow-[0_0_0_1px_rgba(255,255,255,0.08),0_18px_48px_rgba(0,0,0,0.34),inset_0_1px_0_rgba(255,255,255,0.04)] backdrop-blur-2xl">
              <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(255,255,255,0.05),_transparent_24%),radial-gradient(circle_at_78%_22%,_rgba(255,255,255,0.035),_transparent_20%)]" />
              <div className="absolute inset-x-0 top-0 h-24 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),transparent)]" />
              <div className="absolute inset-x-8 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.34),transparent)]" />

              <div className="relative border-b border-white/8 px-5 py-4 sm:px-6">
                <div className="flex flex-col gap-4">
                  <div className="flex flex-wrap items-center justify-between gap-3 text-[11px] uppercase tracking-[0.14em] text-white/36">
                    <div className="flex items-center gap-3">
                      <div className="flex items-center gap-1.5">
                        <span className="size-2 rounded-full bg-white/24" />
                        <span className="size-2 rounded-full bg-white/14" />
                        <span className="size-2 rounded-full bg-white/42" />
                      </div>
                      <span>{t.theater.liveLabel}</span>
                    </div>
                    <div className="inline-flex items-center gap-2 text-white/52">
                      <span className="tabular-nums">
                        {String(activeIndex + 1).padStart(2, "0")}/
                        {String(steps.length).padStart(2, "0")}
                      </span>
                      <span className="h-1 w-1 rounded-full bg-white/55" />
                      <span className="text-white/72">{activeStep.signal}</span>
                    </div>
                  </div>

                  <div>
                    <div className="text-[10px] uppercase tracking-[0.16em] text-white/34">
                      {t.theater.panelTaskLabel}
                    </div>
                    <div className="mt-2 max-w-[38rem] text-[15px] font-medium leading-[1.65] tracking-[-0.02em] text-white/90 sm:text-[16px]">
                      {t.theater.panelTaskValue}
                    </div>

                    <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px] uppercase tracking-[0.12em] text-white/38">
                      <div className="inline-flex items-center gap-2">
                        <span>{t.theater.panelQueueLabel}</span>
                        <span className="text-white/74">{t.theater.panelQueueValue}</span>
                      </div>

                      <div className="inline-flex items-center gap-2">
                        <span>{t.theater.panelRuntimeLabel}</span>
                        <span className="inline-flex items-center gap-1.5 text-white/74">
                          <CodexLogo className="size-3.5 text-white/72" />
                          <span>codex</span>
                        </span>
                        <span className="text-white/22">/</span>
                        <span className="inline-flex items-center gap-1.5 text-white/74">
                          <ClaudeCodeLogo className="size-3.5 text-white/68" />
                          <span>claude</span>
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div className="relative px-5 py-4 sm:px-6 sm:py-5">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
                  <p
                    key={`${activeStep.id}-feed`}
                    className="animate-in fade-in-0 slide-in-from-bottom-1 max-w-[36rem] text-[12px] leading-[1.7] text-white/52 duration-500"
                  >
                    {activeStep.statusValue}
                  </p>
                  <div className="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-white/42">
                    <span className="tabular-nums text-white/64">
                      {String(activeIndex + 1).padStart(2, "0")}
                    </span>
                    <span className="h-1 w-1 rounded-full bg-white/44" />
                    <span>{activeStep.label}</span>
                  </div>
                </div>

                <div className="mt-3 h-px w-full overflow-hidden rounded-full bg-white/8">
                  <div
                    className="h-full rounded-full bg-[linear-gradient(90deg,rgba(255,255,255,0.18),rgba(255,255,255,0.94),rgba(255,255,255,0.22))] transition-all duration-700 ease-out"
                    style={{ width: sceneProgress }}
                  />
                </div>

                <div
                  className="relative mt-4 h-[360px] overflow-hidden rounded-[28px] border border-white/8 bg-[linear-gradient(180deg,rgba(7,10,15,0.94),rgba(3,5,10,1))] shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] sm:h-[420px]"
                  aria-label={t.theater.sceneAriaLabel}
                >
                  <LandingProofScene activeIndex={activeIndex} />
                  <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.04),transparent_26%),linear-gradient(180deg,rgba(255,255,255,0.02),transparent_18%,transparent_82%,rgba(255,255,255,0.02))]" />
                  <div className="absolute inset-[1px] rounded-[27px] border border-white/6" />

                  <div
                    className="pointer-events-none absolute z-[1] h-[170px] w-[170px] rounded-full bg-white/8 blur-[92px] transition-all duration-700 ease-out"
                    style={{
                      left:
                        sceneNodePositions[activeIndex]?.left ??
                        fallbackSceneNodePosition.left,
                      top:
                        sceneNodePositions[activeIndex]?.top ??
                        fallbackSceneNodePosition.top,
                      transform: "translate(-50%, -50%)",
                    }}
                  />
                  <div
                    className="pointer-events-none absolute z-[1] h-[130px] w-[130px] rounded-full bg-white/5 blur-[76px] transition-all duration-700 ease-out"
                    style={{
                      left:
                        sceneNodePositions[Math.min(activeIndex + 1, steps.length - 1)]
                          ?.left ?? fallbackSceneNodePosition.left,
                      top:
                        sceneNodePositions[Math.min(activeIndex + 1, steps.length - 1)]
                          ?.top ?? fallbackSceneNodePosition.top,
                      transform: "translate(-50%, -50%)",
                    }}
                  />

                  <svg
                    className="pointer-events-none absolute inset-0 z-[2] h-full w-full"
                    viewBox="0 0 100 100"
                    preserveAspectRatio="none"
                    aria-hidden="true"
                  >
                    <defs>
                      <linearGradient
                        id="landing-flow-path"
                        x1="0%"
                        y1="0%"
                        x2="100%"
                        y2="0%"
                      >
                        <stop offset="0%" stopColor="#ffffff" stopOpacity="0.12" />
                        <stop offset="55%" stopColor="#ffffff" stopOpacity="0.92" />
                        <stop offset="100%" stopColor="#ffffff" stopOpacity="0.18" />
                      </linearGradient>
                    </defs>
                    <path
                      d="M 8 18 C 15 18, 19 32, 28 64 S 39 44, 49 38 S 61 44, 71 61 S 84 30, 90 24"
                      fill="none"
                      stroke="rgba(7,10,15,0.96)"
                      strokeWidth="7"
                      strokeLinecap="round"
                    />
                    <path
                      d="M 8 18 C 15 18, 19 32, 28 64 S 39 44, 49 38 S 61 44, 71 61 S 84 30, 90 24"
                      fill="none"
                      stroke="url(#landing-flow-path)"
                      strokeWidth="2.6"
                      strokeLinecap="round"
                    />
                  </svg>

                  <div
                    className="pointer-events-none absolute z-[22] transition-all duration-700 ease-out"
                    style={{
                      left:
                        sceneNodePositions[activeIndex]?.left ??
                        fallbackSceneNodePosition.left,
                      top:
                        sceneNodePositions[activeIndex]?.top ??
                        fallbackSceneNodePosition.top,
                      transform: scenePacketTransform,
                    }}
                  >
                    <div className="relative flex h-10 items-center gap-2 rounded-full border border-white/16 bg-white/88 px-3.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-900 shadow-[0_10px_28px_rgba(0,0,0,0.22)]">
                      <div className="absolute inset-x-2 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.7),transparent)]" />
                      <span className="inline-flex size-2 rounded-full bg-slate-900/78" />
                      <span>{t.theater.taskPacketLabel}</span>
                    </div>
                  </div>

                  {steps.map((step, index) => {
                    const nodePosition =
                      sceneNodePositions[index] ?? fallbackSceneNodePosition;

                    return (
                      <button
                        key={step.id}
                        type="button"
                        onClick={() => setActiveIndex(index)}
                        className={cn(
                          "absolute z-10 flex min-w-[112px] -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-full border px-3.5 py-2.5 text-left text-[11px] uppercase tracking-[0.12em] backdrop-blur-xl transition-all",
                          activeIndex === index
                            ? "border-white/16 bg-[linear-gradient(180deg,rgba(37,43,54,0.94),rgba(18,23,31,0.9))] text-white shadow-[0_10px_24px_rgba(0,0,0,0.2)]"
                            : "border-white/8 bg-[linear-gradient(180deg,rgba(13,18,26,0.78),rgba(8,11,18,0.68))] text-white/50 hover:border-white/14 hover:text-white/72",
                        )}
                        style={{
                          left: nodePosition.left,
                          top: nodePosition.top,
                        }}
                        aria-pressed={activeIndex === index}
                      >
                        <span className="inline-flex h-7 min-w-7 items-center justify-center rounded-full border border-current/20 bg-black/12 px-1 text-[10px] font-semibold tabular-nums">
                          {String(index + 1).padStart(2, "0")}
                        </span>
                        <span>{step.label}</span>
                      </button>
                    );
                  })}
                </div>

                <div
                  key={`${activeStep.id}-detail`}
                  className="mt-4 animate-in fade-in-0 slide-in-from-bottom-2 rounded-[24px] bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] px-4 py-4 shadow-[0_0_0_1px_rgba(255,255,255,0.06)] duration-500 backdrop-blur-xl sm:px-5"
                >
                  <div className="grid gap-6 xl:grid-cols-[minmax(0,1.08fr)_minmax(0,0.92fr)]">
                    <div>
                      <div className="text-[11px] uppercase tracking-[0.12em] text-white/36">
                        {t.theater.activeFocusLabel}
                      </div>
                      <div className="mt-3 text-[22px] font-semibold tracking-[-0.045em] text-white/94">
                        {activeStep.title}
                      </div>
                      <p className="mt-2 max-w-[36rem] text-[14px] leading-[1.72] text-white/58">
                        {activeStep.description}
                      </p>

                      <div className="mt-5 border-t border-white/8 pt-4">
                        <div className="text-[11px] uppercase tracking-[0.12em] text-white/34">
                          {t.theater.stageNoteLabel}
                        </div>
                        <div className="mt-2 text-[13px] leading-[1.72] text-white/56">
                          {activeStep.meta}
                        </div>
                      </div>
                    </div>

                    <div className="space-y-4">
                      <div>
                        <div className="text-[11px] uppercase tracking-[0.12em] text-white/34">
                          {activeStep.resultLabel}
                        </div>
                        <div className="mt-2 text-[14px] leading-[1.72] text-white/88">
                          {activeStep.resultValue}
                        </div>
                      </div>

                      <div className="divide-y divide-white/8 border-y border-white/8">
                        {systemFacts.map((fact) => {
                          const Icon = fact.icon;

                          return (
                            <div
                              key={fact.label}
                              className="flex items-start justify-between gap-4 py-3"
                            >
                              <div className="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.12em] text-white/36">
                                <Icon className="mt-0.5 size-3.5 shrink-0" />
                                <span>{fact.label}</span>
                              </div>
                              <div className="max-w-[15rem] text-right text-[13px] leading-[1.62] text-white/76">
                                {fact.value}
                              </div>
                            </div>
                          );
                        })}
                      </div>

                      <div>
                        <div className="text-[11px] uppercase tracking-[0.12em] text-emerald-100/68">
                          {t.theater.panelNextLabel}
                        </div>
                        <div className="mt-2 text-[14px] leading-[1.72] text-emerald-50">
                          {activeStep.nextAction}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
