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
const sceneChipPositions = [
  "left-5 top-5 sm:left-7 sm:top-7",
  "right-6 top-4 sm:right-8 sm:top-8",
  "right-4 top-1/2 -translate-y-1/2 sm:right-6",
  "bottom-6 right-10 sm:bottom-8 sm:right-12",
  "bottom-5 left-6 sm:bottom-8 sm:left-10",
];

export function LandingTheater() {
  const { t } = useLocale();
  const user = useAuthStore((state) => state.user);
  const [activeIndex, setActiveIndex] = useState(0);
  const steps = t.theater.steps;
  const activeStep = steps[activeIndex] ?? steps[0];

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
            <div className="inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1.5 text-[12px] font-medium uppercase tracking-[0.12em] text-cyan-100/88 backdrop-blur">
              <Workflow className="size-3.5" />
              <span>{t.theater.kicker}</span>
            </div>

            <h1 className="mt-4 max-w-[11ch] text-[2.7rem] font-semibold leading-[0.92] tracking-[-0.07em] text-white sm:text-[3.55rem] lg:text-[4rem]">
              {t.theater.headlineLine1}
              <span className="mt-2 block text-white/58">
                {t.theater.headlineLine2}
              </span>
            </h1>

            <p className="mt-4 max-w-[32rem] text-[15px] leading-[1.75] text-white/66 sm:text-[16px]">
              {t.theater.description}
            </p>

            <div className="mt-6 flex flex-wrap gap-3">
              <Link
                href={user ? "/issues" : "/login"}
                className={headerButtonClassName("solid", "dark")}
              >
                {user ? t.header.dashboard : t.theater.primaryCta}
                <ArrowRight className="size-4" />
              </Link>
            </div>

            <div className="mt-4 flex flex-wrap gap-2.5">
              {t.theater.proofChips.map((chip, index) => {
                const Icon = proofIcons[index] ?? Sparkles;

                return (
                  <div
                    key={chip}
                    className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-2 text-[13px] text-white/72 backdrop-blur-md"
                  >
                    <Icon className="size-3.5 text-cyan-200" />
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
            <div className="absolute -left-12 top-12 h-36 w-36 rounded-full bg-cyan-300/12 blur-3xl" />
            <div className="absolute -right-10 bottom-10 h-32 w-32 rounded-full bg-emerald-300/10 blur-3xl" />

            <div className="relative overflow-hidden rounded-[32px] border border-white/10 bg-[linear-gradient(180deg,rgba(15,23,42,0.84),rgba(3,7,18,0.96))] shadow-[0_36px_140px_rgba(2,8,23,0.62)]">
              <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(56,189,248,0.12),_transparent_34%)]" />
              <div className="absolute inset-x-0 top-0 h-24 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),transparent)]" />

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

                <div className="relative mt-4 h-[260px] overflow-hidden rounded-[28px] border border-white/8 bg-[linear-gradient(180deg,rgba(5,10,22,0.86),rgba(4,8,18,0.96))] shadow-[inset_0_1px_0_rgba(255,255,255,0.05)] sm:h-[300px]">
                  <LandingProofScene activeIndex={activeIndex} />

                  {steps.map((step, index) => (
                    <button
                      key={step.id}
                      type="button"
                      onClick={() => setActiveIndex(index)}
                      className={cn(
                        "absolute rounded-full border px-3 py-1.5 text-left text-[11px] uppercase tracking-[0.12em] backdrop-blur-md transition-all",
                        sceneChipPositions[index] ?? sceneChipPositions[0],
                        activeIndex === index
                          ? "border-cyan-300/40 bg-cyan-300/14 text-cyan-100 shadow-[0_0_24px_rgba(34,211,238,0.15)]"
                          : "border-white/10 bg-black/30 text-white/54 hover:border-white/18 hover:text-white/76",
                      )}
                      aria-pressed={activeIndex === index}
                    >
                      {step.label}
                    </button>
                  ))}

                  <div className="pointer-events-none absolute inset-x-4 bottom-4 grid gap-3 sm:inset-x-5 sm:grid-cols-[minmax(0,1fr)_auto]">
                    <div className="rounded-[20px] border border-white/8 bg-black/34 px-4 py-3 backdrop-blur-md">
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
