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

  const metrics = [
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
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,_rgba(56,189,248,0.18),_transparent_34%),radial-gradient(circle_at_88%_15%,_rgba(15,23,42,0.9),_transparent_24%),radial-gradient(circle_at_74%_72%,_rgba(34,197,94,0.1),_transparent_22%),linear-gradient(180deg,_#08111d_0%,_#04070d_58%,_#02040a_100%)]" />
      <div className="absolute inset-0 bg-[linear-gradient(rgba(148,163,184,0.06)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.06)_1px,transparent_1px)] [background-size:72px_72px] opacity-30" />
      <div className="absolute inset-x-0 top-0 h-px bg-white/12" />

      <section className="relative mx-auto max-w-[1360px] px-4 pb-14 pt-16 sm:px-6 sm:pb-16 sm:pt-20 lg:px-8 lg:pb-[4.5rem]">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,500px)_1fr] lg:items-center">
          <div className="relative z-10">
            <div className="inline-flex items-center gap-2 rounded-full border border-cyan-300/18 bg-cyan-300/[0.08] px-3 py-1.5 text-[11px] font-medium uppercase tracking-[0.18em] text-cyan-100/80 backdrop-blur">
              <Workflow className="size-3.5" />
              <span>{t.theater.kicker}</span>
            </div>

            <h1
              className={cn(
                "mt-5 text-[2.7rem] font-semibold leading-[0.9] tracking-[-0.08em] text-white sm:text-[3.6rem] lg:text-[4.15rem]",
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

            <div className="mt-5 max-w-[34rem] rounded-[24px] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.04),rgba(255,255,255,0.02))] p-5 shadow-[0_18px_48px_rgba(2,8,23,0.16)] backdrop-blur-xl">
              <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-white/36">
                <span className="inline-flex h-px w-6 bg-cyan-200/50" />
                <span>{t.theater.stepLabel}</span>
              </div>
              <p className="mt-3 text-[15px] leading-[1.8] text-white/68 sm:text-[16px]">
                {t.theater.description}
              </p>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <Link
                href={user ? "/issues" : "/login"}
                className={headerButtonClassName("solid", "dark")}
              >
                {user ? t.header.dashboard : t.theater.primaryCta}
                <ArrowRight className="size-4" />
              </Link>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3 py-2 text-[12px] text-white/54 backdrop-blur-md">
                <span className="inline-flex size-1.5 rounded-full bg-cyan-300" />
                <span>{activeStep.signal}</span>
              </div>
            </div>

            <div className="mt-5 grid gap-2.5 sm:grid-cols-2">
              {t.theater.proofChips.map((chip, index) => {
                const Icon = proofIcons[index] ?? Sparkles;

                return (
                  <div
                    key={chip}
                    className={cn(
                      "inline-flex items-start gap-2 rounded-[18px] border border-white/10 bg-white/[0.04] px-3.5 py-3 text-[13px] leading-[1.5] text-white/70 backdrop-blur-md",
                      index === 2 ? "sm:col-span-2" : "",
                    )}
                  >
                    <Icon className="mt-0.5 size-3.5 shrink-0 text-cyan-200" />
                    <span>{chip}</span>
                  </div>
                );
              })}
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-3 text-[13px] text-white/48">
              <span>{t.theater.worksWith}</span>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/6 px-3 py-1.5 text-white/74">
                <ClaudeCodeLogo className="size-4" />
                <span>Claude Code</span>
              </div>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/6 px-3 py-1.5 text-white/74">
                <CodexLogo className="size-4" />
                <span>Codex</span>
              </div>
            </div>
          </div>

          <div className="relative">
            <div className="absolute -left-12 top-12 h-32 w-32 rounded-full bg-cyan-300/10 blur-3xl" />
            <div className="absolute -right-10 bottom-10 h-28 w-28 rounded-full bg-emerald-300/8 blur-3xl" />

            <div className="relative overflow-hidden rounded-[32px] border border-white/12 bg-[linear-gradient(180deg,rgba(15,23,42,0.78),rgba(5,10,22,0.94))] shadow-[0_24px_90px_rgba(2,8,23,0.48)] backdrop-blur-xl">
              <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(255,255,255,0.06),_transparent_26%),radial-gradient(circle_at_78%_22%,_rgba(56,189,248,0.08),_transparent_22%)]" />
              <div className="absolute inset-x-0 top-0 h-24 bg-[linear-gradient(180deg,rgba(255,255,255,0.09),transparent)]" />
              <div className="absolute inset-x-8 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.46),transparent)]" />

              <div className="relative border-b border-white/8 px-5 py-3.5 sm:px-6">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <div className="text-[12px] font-medium uppercase tracking-[0.12em] text-white/38">
                      {t.theater.liveLabel}
                    </div>
                    <div className="mt-2 text-[20px] font-semibold tracking-[-0.03em] text-white/94">
                      {activeStep.title}
                    </div>
                    <p className="mt-1.5 max-w-[38rem] text-[13px] leading-[1.65] text-white/56">
                      {activeStep.description}
                    </p>
                  </div>

                  <div className="inline-flex items-center gap-2 rounded-full border border-emerald-300/24 bg-emerald-300/10 px-3 py-1.5 text-[12px] font-medium text-emerald-100">
                    <CheckCircle2 className="size-3.5" />
                    <span>{activeStep.signal}</span>
                  </div>
                </div>
              </div>

              <div
                className="relative grid gap-3 border-b border-white/8 px-5 py-3.5 sm:px-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)_auto]"
                aria-label={t.theater.sceneAriaLabel}
              >
                <div className="rounded-[22px] border border-white/8 bg-white/[0.045] px-4 py-3.5 backdrop-blur-md">
                  <div className="text-[11px] uppercase tracking-[0.12em] text-white/38">
                    {t.theater.panelTaskLabel}
                  </div>
                  <div className="mt-2 text-[16px] font-medium leading-[1.45] text-white/92">
                    {t.theater.panelTaskValue}
                  </div>
                </div>

                <div className="rounded-[22px] border border-white/8 bg-white/[0.045] px-4 py-3.5 backdrop-blur-md">
                  <div className="text-[11px] uppercase tracking-[0.12em] text-white/38">
                    {t.theater.panelQueueLabel}
                  </div>
                  <div className="mt-2 text-[14px] font-medium text-white/88">
                    {t.theater.panelQueueValue}
                  </div>
                </div>

                <div className="rounded-[22px] border border-white/8 bg-white/[0.045] px-4 py-3.5 backdrop-blur-md">
                  <div className="text-[11px] uppercase tracking-[0.12em] text-white/38">
                    {t.theater.panelRuntimeLabel}
                  </div>
                  <div className="mt-2.5 space-y-1.5 text-[13px] text-white/78">
                    <div className="flex items-center gap-2">
                      <CodexLogo className="size-4 text-cyan-200" />
                      <span>codex</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <ClaudeCodeLogo className="size-4 text-amber-100" />
                      <span>claude</span>
                    </div>
                  </div>
                </div>
              </div>

              <div className="relative px-5 py-4 sm:px-6 sm:py-[1.125rem]">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                  <div>
                    <div className="text-[12px] font-medium uppercase tracking-[0.12em] text-white/38">
                      {t.theater.cycleLabel}
                    </div>
                    <p className="mt-1.5 max-w-[36rem] text-[13px] leading-[1.65] text-white/54">
                      {t.theater.cycleHint}
                    </p>
                  </div>
                  <div className="inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1.5 text-[12px] font-medium text-cyan-100/88">
                    <span className="tabular-nums">
                      {String(activeIndex + 1).padStart(2, "0")}/
                      {String(steps.length).padStart(2, "0")}
                    </span>
                    <span>{activeStep.label}</span>
                  </div>
                </div>

                <div className="mt-3 h-px w-full overflow-hidden rounded-full bg-white/8">
                  <div
                    className="h-full rounded-full bg-[linear-gradient(90deg,rgba(56,189,248,0.45),rgba(224,242,254,0.95),rgba(103,232,249,0.4))] transition-all duration-700 ease-out"
                    style={{ width: sceneProgress }}
                  />
                </div>

                <div className="relative mt-4 h-[320px] overflow-hidden rounded-[28px] border border-white/8 bg-[linear-gradient(180deg,rgba(8,14,28,0.9),rgba(5,9,18,0.98))] shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] sm:h-[360px]">
                  <LandingProofScene activeIndex={activeIndex} />
                  <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.06),transparent_28%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_18%,transparent_82%,rgba(255,255,255,0.03))]" />
                  <div className="absolute inset-[1px] rounded-[27px] border border-white/6" />

                  <div
                    className="pointer-events-none absolute z-[1] h-[190px] w-[190px] rounded-full bg-cyan-300/14 blur-[84px] transition-all duration-700 ease-out"
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
                    className="pointer-events-none absolute z-[1] h-[150px] w-[150px] rounded-full bg-white/7 blur-[72px] transition-all duration-700 ease-out"
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
                        <stop offset="0%" stopColor="#38bdf8" stopOpacity="0.2" />
                        <stop offset="55%" stopColor="#d6f4ff" stopOpacity="0.96" />
                        <stop offset="100%" stopColor="#67e8f9" stopOpacity="0.28" />
                      </linearGradient>
                    </defs>
                    <path
                      d="M 8 18 C 15 18, 19 32, 28 64 S 39 44, 49 38 S 61 44, 71 61 S 84 30, 90 24"
                      fill="none"
                      stroke="rgba(14,28,48,0.98)"
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
                    <div className="relative flex h-10 items-center gap-2 rounded-full border border-white/22 bg-white/92 px-3.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-900 shadow-[0_16px_40px_rgba(2,8,23,0.26)]">
                      <div className="absolute inset-x-2 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(56,189,248,0.8),transparent)]" />
                      <span className="inline-flex size-2 rounded-full bg-cyan-500 shadow-[0_0_18px_rgba(34,211,238,0.8)]" />
                      <span>{t.theater.taskPacketLabel}</span>
                    </div>
                  </div>

                  <div className="pointer-events-none absolute left-4 top-4 z-20 max-w-[168px] rounded-[20px] border border-white/10 bg-[linear-gradient(180deg,rgba(11,19,33,0.78),rgba(7,12,24,0.62))] px-4 py-3 backdrop-blur-xl shadow-[0_18px_48px_rgba(2,8,23,0.24)]">
                    <div className="text-[10px] uppercase tracking-[0.18em] text-white/38">
                      {t.theater.activeFocusLabel}
                    </div>
                    <div className="mt-2 flex items-center gap-2 text-[13px] font-medium text-white/88">
                      <span className="inline-flex h-6 min-w-6 items-center justify-center rounded-full border border-cyan-300/30 bg-cyan-300/12 px-1 text-[10px] font-semibold text-cyan-100">
                        {String(activeIndex + 1).padStart(2, "0")}
                      </span>
                      <span>{activeStep.label}</span>
                    </div>
                    <div className="mt-2 max-w-[180px] text-[12px] leading-[1.55] text-white/52">
                      {activeStep.resultValue}
                    </div>
                  </div>

                  <div className="pointer-events-none absolute bottom-4 left-4 z-20 max-w-[260px] rounded-[18px] border border-white/10 bg-[linear-gradient(180deg,rgba(9,16,30,0.72),rgba(7,12,24,0.56))] px-4 py-3 backdrop-blur-xl shadow-[0_18px_44px_rgba(2,8,23,0.22)]">
                    <div className="text-[10px] uppercase tracking-[0.18em] text-white/36">
                      {t.theater.stageNoteLabel}
                    </div>
                    <div className="mt-2 text-[12px] leading-[1.6] text-white/58">
                      {activeStep.meta}
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
                          "absolute z-10 flex min-w-[118px] -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-full border px-3.5 py-2.5 text-left text-[11px] uppercase tracking-[0.12em] backdrop-blur-xl transition-all",
                          activeIndex === index
                            ? "border-cyan-200/28 bg-[linear-gradient(180deg,rgba(18,36,58,0.88),rgba(9,18,32,0.8))] text-sky-50 shadow-[0_14px_36px_rgba(15,23,42,0.28)]"
                            : "border-white/10 bg-[linear-gradient(180deg,rgba(8,16,29,0.72),rgba(6,12,24,0.62))] text-white/58 hover:border-white/18 hover:text-white/78",
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

                <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
                  <div className="rounded-[20px] border border-white/8 bg-black/28 px-4 py-3 backdrop-blur-md">
                    <div className="text-[11px] uppercase tracking-[0.12em] text-white/40">
                      {activeStep.statusLabel}
                    </div>
                    <div className="mt-2 text-[17px] font-medium text-white/92">
                      {activeStep.statusValue}
                    </div>
                  </div>

                  <div className="rounded-[20px] border border-emerald-300/18 bg-emerald-300/10 px-4 py-3 text-[13px] text-emerald-50 backdrop-blur-md">
                    {activeStep.nextAction}
                  </div>
                </div>

                <div className="mt-4 grid gap-3 lg:grid-cols-3">
                  {metrics.map((metric) => {
                    const Icon = metric.icon;

                    return (
                      <div
                        key={metric.label}
                        className="rounded-[22px] border border-white/8 bg-white/[0.04] px-4 py-4 backdrop-blur-md"
                      >
                        <div className="flex items-center gap-2 text-[12px] font-medium uppercase tracking-[0.12em] text-white/40">
                          <Icon className="size-3.5" />
                          <span>{metric.label}</span>
                        </div>
                        <div className="mt-3 text-[14px] leading-[1.7] text-white/86">
                          {metric.value}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}
