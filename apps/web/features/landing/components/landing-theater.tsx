"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ArrowRight, CheckCircle2, Orbit, Play } from "lucide-react";
import { useAuthStore } from "@/features/auth";
import { cn } from "@/lib/utils";
import { useLocale } from "../i18n";
import {
  ClaudeCodeLogo,
  CodexLogo,
  GitHubMark,
  githubUrl,
  headerButtonClassName,
} from "./shared";
import { LandingFlowScene } from "./landing-flow-scene";

const LOOP_INTERVAL_MS = 4800;

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

  return (
    <main
      id="product"
      className="relative overflow-hidden bg-[#05070b] text-white"
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,_rgba(56,189,248,0.2),_transparent_34%),radial-gradient(circle_at_85%_18%,_rgba(245,158,11,0.16),_transparent_24%),linear-gradient(180deg,_#081019_0%,_#05070b_55%,_#030508_100%)]" />
      <div className="absolute inset-x-0 top-0 h-px bg-white/12" />

      <section className="relative mx-auto max-w-[1320px] px-4 pb-16 pt-28 sm:px-6 sm:pb-20 sm:pt-32 lg:px-8 lg:pb-24">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,540px)_1fr] lg:items-center">
          <div className="relative z-10">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/12 bg-white/6 px-3 py-1.5 text-[12px] font-medium tracking-[0.08em] text-white/72 uppercase backdrop-blur">
              <Orbit className="size-3.5" />
              <span>{t.theater.kicker}</span>
            </div>

            <h1 className="mt-6 max-w-[11ch] font-[family-name:var(--font-serif)] text-[3.25rem] leading-[0.9] tracking-[-0.05em] text-white sm:text-[4.5rem] lg:text-[5.6rem]">
              {t.theater.headlineLine1}
              <span className="block text-white/56">
                {t.theater.headlineLine2}
              </span>
            </h1>

            <p className="mt-6 max-w-[33rem] text-[16px] leading-[1.8] text-white/64 sm:text-[17px]">
              {t.theater.description}
            </p>

            <div className="mt-8 flex flex-wrap gap-3">
              <Link
                href={user ? "/issues" : "/login"}
                className={headerButtonClassName("solid", "dark")}
              >
                {user ? t.header.dashboard : t.theater.primaryCta}
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href={githubUrl}
                target="_blank"
                rel="noreferrer"
                className={headerButtonClassName("ghost", "dark")}
              >
                <GitHubMark className="size-4" />
                {t.theater.secondaryCta}
              </Link>
            </div>

            <div className="mt-8 flex flex-wrap items-center gap-3 text-[13px] text-white/48">
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

            <div className="mt-12">
              <div className="mb-4 text-[12px] font-medium uppercase tracking-[0.12em] text-white/38">
                {t.theater.stepLabel}
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                {steps.map((step, index) => (
                  <button
                    key={step.id}
                    type="button"
                    onClick={() => setActiveIndex(index)}
                    className={cn(
                      "group rounded-[18px] border px-4 py-3 text-left transition-all",
                      activeIndex === index
                        ? "border-cyan-300/40 bg-cyan-300/12 shadow-[0_0_0_1px_rgba(103,232,249,0.15)]"
                        : "border-white/8 bg-white/[0.03] hover:border-white/18 hover:bg-white/[0.05]",
                    )}
                    aria-pressed={activeIndex === index}
                  >
                    <div className="text-[11px] font-medium uppercase tracking-[0.12em] text-white/36">
                      {String(index + 1).padStart(2, "0")}
                    </div>
                    <div className="mt-1 text-[17px] font-medium text-white/92">
                      {step.label}
                    </div>
                    <div className="mt-1 text-[13px] leading-[1.6] text-white/48">
                      {step.meta}
                    </div>
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div className="relative">
            <div className="relative overflow-hidden rounded-[28px] border border-white/10 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.03))] shadow-[0_30px_120px_rgba(3,7,18,0.75)]">
              <div className="absolute inset-x-0 top-0 h-20 bg-[linear-gradient(180deg,rgba(255,255,255,0.12),transparent)]" />
              <div className="grid gap-4 border-b border-white/8 px-5 py-4 sm:grid-cols-[1fr_auto] sm:items-center sm:px-6">
                <div>
                  <div className="text-[12px] font-medium uppercase tracking-[0.12em] text-white/36">
                    {t.theater.liveLabel}
                  </div>
                  <div className="mt-1 text-[18px] font-medium text-white/92">
                    {activeStep.title}
                  </div>
                </div>
                <div className="inline-flex items-center gap-2 rounded-full border border-emerald-300/20 bg-emerald-300/10 px-3 py-1.5 text-[12px] font-medium text-emerald-100">
                  <CheckCircle2 className="size-3.5" />
                  <span>{activeStep.statusValue}</span>
                </div>
              </div>

              <div className="relative h-[440px] sm:h-[520px]" aria-label={t.theater.sceneAriaLabel}>
                <LandingFlowScene activeIndex={activeIndex} />

                <div className="pointer-events-none absolute inset-x-4 top-4 flex items-start justify-between gap-4 sm:inset-x-6 sm:top-5">
                  <div className="rounded-[18px] border border-white/10 bg-black/35 px-4 py-3 backdrop-blur-md">
                    <div className="flex items-center gap-2 text-[12px] font-medium uppercase tracking-[0.12em] text-white/42">
                      <Play className="size-3" />
                      <span>{activeStep.label}</span>
                    </div>
                    <p className="mt-2 max-w-[15rem] text-[13px] leading-[1.6] text-white/68">
                      {activeStep.description}
                    </p>
                  </div>

                  <div className="hidden rounded-[18px] border border-white/10 bg-black/30 px-3 py-3 backdrop-blur-md sm:block">
                    <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.12em] text-white/38">
                      <span>Runtime</span>
                    </div>
                    <div className="mt-2 space-y-2 text-[12px] text-white/70">
                      <div className="flex items-center gap-2">
                        <CodexLogo className="size-3.5 text-cyan-200" />
                        <span>codex</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <ClaudeCodeLogo className="size-3.5 text-amber-100" />
                        <span>claude</span>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-x-4 bottom-4 grid gap-3 sm:inset-x-6 sm:bottom-6 sm:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
                  <div className="rounded-[22px] border border-white/10 bg-black/38 px-5 py-4 backdrop-blur-md">
                    <div className="text-[12px] font-medium uppercase tracking-[0.12em] text-white/38">
                      {activeStep.statusLabel}
                    </div>
                    <div className="mt-2 text-[18px] font-medium text-white/94">
                      {activeStep.statusValue}
                    </div>
                    <div className="mt-3 text-[13px] leading-[1.7] text-white/60">
                      {activeStep.description}
                    </div>
                  </div>

                  <div className="rounded-[22px] border border-white/10 bg-black/38 px-5 py-4 backdrop-blur-md">
                    <div className="text-[12px] font-medium uppercase tracking-[0.12em] text-white/38">
                      {activeStep.resultLabel}
                    </div>
                    <div className="mt-2 text-[18px] font-medium text-white/94">
                      {activeStep.resultValue}
                    </div>
                    <div className="mt-3 text-[13px] leading-[1.7] text-white/60">
                      {activeStep.meta}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-10 grid gap-3 sm:grid-cols-3">
          {t.theater.proofChips.map((chip) => (
            <div
              key={chip}
              className="rounded-[18px] border border-white/8 bg-white/[0.04] px-4 py-4 text-[14px] text-white/70 backdrop-blur"
            >
              {chip}
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
