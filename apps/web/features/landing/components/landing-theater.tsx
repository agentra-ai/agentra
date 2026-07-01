"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  ArrowRight,
  Bot,
  CheckCircle2,
  GitBranch,
  ShieldCheck,
  Sparkles,
  Workflow,
} from "lucide-react";
import gsap from "gsap";
import { useAuthStore } from "@/features/auth";
import { cn } from "@/lib/utils";
import {
  ClaudeCodeLogo,
  CodexLogo,
  headerButtonClassName,
} from "./shared";

const LOOP_INTERVAL_MS = 5500;
const proofIcons = [Workflow, ShieldCheck, Sparkles];

type TheaterStep = {
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
};

/**
 * Central SVG hero — a stylized "current task" card with five step
 * indicators beneath. The active step pulses via GSAP; completed
 * steps render a checkmark; pending steps are dots.
 */
function StepHero({
  activeIndex,
  steps,
}: {
  activeIndex: number;
  steps: TheaterStep[];
}) {
  const pulseRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const node = pulseRef.current;
    if (!node) {
      return;
    }
    const ctx = gsap.context(() => {
      gsap.fromTo(
        node,
        { scale: 0.6, autoAlpha: 0 },
        { scale: 1, autoAlpha: 1, duration: 0.5, ease: "power2.out" },
      );
    }, node);
    return () => ctx.revert();
  }, [activeIndex]);

  return (
    <div className="flex flex-col items-center gap-8">
      <div
        ref={pulseRef}
        className="relative w-[260px] sm:w-[280px]"
      >
        <div
          className="absolute -inset-6 rounded-[28px] bg-gradient-to-b from-white/[0.07] to-transparent blur-2xl"
          aria-hidden="true"
        />
        <div
          className="relative rounded-[18px] border border-white/[0.08] bg-[linear-gradient(180deg,rgba(255,255,255,0.06),rgba(255,255,255,0.02))] p-5 shadow-[0_18px_48px_rgba(0,0,0,0.34),inset_0_1px_0_rgba(255,255,255,0.05)] backdrop-blur-xl"
        >
          <div className="flex items-center justify-between text-[10px] uppercase tracking-[0.18em] text-white/36">
            <span>Current task</span>
            <span className="inline-flex items-center gap-1.5 text-white/52">
              <span
                className="size-1.5 rounded-full bg-emerald-400"
                style={{ boxShadow: "0 0 0 4px rgba(52,211,153,0.18)" }}
              />
              <span>{steps[activeIndex]?.label}</span>
            </span>
          </div>

          <div className="mt-3 flex items-center gap-2.5">
            <Bot className="size-6 text-white/72" />
            <div className="flex-1">
              <div className="h-2.5 w-full rounded-full bg-white/12" />
              <div className="mt-2 h-2.5 w-3/5 rounded-full bg-white/8" />
            </div>
          </div>

          <div className="mt-4 grid grid-cols-3 gap-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="h-8 rounded-lg bg-white/[0.04] border border-white/[0.06]"
              />
            ))}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-2.5" role="tablist" aria-label="Workflow steps">
        {steps.map((step, index) => {
          const state =
            index < activeIndex
              ? "completed"
              : index === activeIndex
                ? "active"
                : "pending";
          return (
            <div
              key={step.id}
              className="flex flex-col items-center gap-1.5"
            >
              <div
                className={cn(
                  "flex h-9 w-9 items-center justify-center rounded-full border text-[11px] font-semibold tabular-nums transition-all",
                  state === "completed" &&
                    "border-emerald-400/30 bg-emerald-400/15 text-emerald-300",
                  state === "active" &&
                    "border-white/30 bg-white/15 text-white shadow-[0_0_18px_rgba(255,255,255,0.25)]",
                  state === "pending" &&
                    "border-white/10 bg-white/[0.03] text-white/35",
                )}
              >
                {state === "completed" ? (
                  <CheckCircle2 className="size-4" />
                ) : (
                  String(index + 1).padStart(2, "0")
                )}
              </div>
              <span
                className={cn(
                  "text-[10px] uppercase tracking-[0.14em]",
                  state === "active" ? "text-white/70" : "text-white/30",
                )}
              >
                {step.label}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function LandingTheater() {
  const t = useTranslations("landing.theater");
  const tHeader = useTranslations("landing.header");
  const locale = useLocale();
  const user = useAuthStore((state) => state.user);
  const [activeIndex, setActiveIndex] = useState(0);
  const steps = t.raw("steps") as TheaterStep[];
  const activeStep = steps[activeIndex] ?? steps[0];
  const heroRef = useRef<HTMLDivElement | null>(null);
  const headlineWidthClass =
    locale === "zh-CN"
      ? "max-w-[13.2ch] sm:max-w-[13.8ch] lg:max-w-[14.4ch]"
      : "max-w-[11ch]";
  const headlineSecondaryFontClass =
    locale === "zh-CN"
      ? "font-[family-name:var(--font-serif-zh)]"
      : "font-[family-name:var(--font-serif)]";
  const headlineSecondaryClass =
    locale === "zh-CN"
      ? "text-[0.9em] leading-[1.02] tracking-[-0.06em] text-white/72"
      : "tracking-[-0.05em] text-white/70";

  useEffect(() => {
    if (steps.length === 0) {
      return;
    }
    const intervalId = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % steps.length);
    }, LOOP_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [steps.length]);

  // Gentle "breathing" animation on the hero wrapper (scale 1 → 1.02 → 1)
  useEffect(() => {
    const node = heroRef.current;
    if (!node) {
      return;
    }
    const ctx = gsap.context(() => {
      gsap.to(node, {
        scale: 1.02,
        duration: 2,
        ease: "sine.inOut",
        yoyo: true,
        repeat: -1,
      });
    }, node);
    return () => ctx.revert();
  }, []);

  if (!activeStep) {
    return null;
  }

  const systemFacts = [
    {
      label: t("panelOwnerLabel"),
      value: activeStep.owner,
      icon: Bot,
    },
    {
      label: t("panelReviewLabel"),
      value: activeStep.review,
      icon: CheckCircle2,
    },
    {
      label: t("panelArtifactLabel"),
      value: activeStep.artifact,
      icon: GitBranch,
    },
  ];

  return (
    <main
      id="product"
      className="relative overflow-hidden bg-landing-bg text-white"
    >
      {/* CSS gradient background — warm peach top → cool teal bottom */}
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_18%_12%,_rgba(255,213,178,0.22),_transparent_42%),radial-gradient(ellipse_at_82%_88%,_rgba(125,216,232,0.14),_transparent_42%),linear-gradient(160deg,_#1a1410_0%,_#0f1518_42%,_#080b0f_100%)]" />

      <section className="relative mx-auto max-w-[1360px] px-4 pb-14 pt-16 sm:px-6 sm:pb-16 sm:pt-20 lg:px-8 lg:pb-[4.5rem]">
        <div className="grid gap-14 lg:grid-cols-[minmax(0,500px)_1fr] lg:items-start">
          {/* ── Left column ─────────────────────────────────────────────── */}
          <div className="relative z-10">
            <div className="inline-flex items-center gap-3 text-[11px] font-medium uppercase tracking-[0.2em] text-white/58">
              <span className="inline-flex h-px w-10 bg-[linear-gradient(90deg,rgba(255,255,255,0.42),rgba(255,255,255,0))]" />
              <Workflow className="size-3.5 text-white/62" />
              <span>{t("kicker")}</span>
            </div>

            <h1
              className={cn(
                "mt-5 text-[2.85rem] font-semibold leading-[0.88] tracking-[-0.085em] text-white sm:text-[3.75rem] lg:text-[4.35rem]",
                headlineWidthClass,
              )}
            >
              <span className="block">{t("headlineLine1")}</span>
              <span
                className={cn(
                  "mt-2 block",
                  headlineSecondaryFontClass,
                  headlineSecondaryClass,
                )}
              >
                {t("headlineLine2")}
              </span>
            </h1>

            <p className="mt-6 max-w-[35rem] text-[15px] leading-[1.95] text-white/62 sm:text-[16px]">
              {t("description")}
            </p>

            <div className="mt-7 flex flex-wrap items-center gap-3">
              <Link
                href={user ? "/issues" : "/login"}
                className={headerButtonClassName("solid", "dark")}
              >
                {user ? tHeader("dashboard") : t("primaryCta")}
                <ArrowRight className="size-4" />
              </Link>
              <Link
                href="https://github.com/agentra-ai/agentra"
                target="_blank"
                rel="noreferrer"
                className={headerButtonClassName("ghost", "dark")}
              >
                {t("secondaryCta")}
              </Link>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3 py-2 text-[12px] text-white/50 backdrop-blur-md">
                <span className="inline-flex size-1.5 rounded-full bg-white/68" />
                <span>{activeStep.signal}</span>
              </div>
            </div>

            <div className="mt-8 flex items-center gap-3 text-[11px] uppercase tracking-[0.18em] text-white/34">
              <span>{t("stepLabel")}</span>
              <span className="h-px flex-1 bg-white/8" />
            </div>

            <div className="mt-8 max-w-[34rem] divide-y divide-white/8 border-y border-white/8">
              {(t.raw("proofChips") as string[]).map((chip, index) => {
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
              <span>{t("worksWith")}</span>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-white/70">
                <ClaudeCodeLogo className="size-4 text-white/72" />
                <span className="text-[12px] font-medium">Claude Code</span>
              </div>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.045] px-3 py-1.5 text-white/70">
                <CodexLogo className="size-4 text-white/72" />
                <span className="text-[12px] font-medium">Codex</span>
              </div>
            </div>
          </div>

          {/* ── Right column — hero + detail panel ─────────────────────── */}
          <div className="relative">
            <div className="absolute -left-12 top-12 h-32 w-32 rounded-full bg-orange-300/10 blur-3xl" />
            <div className="absolute -right-10 bottom-10 h-28 w-28 rounded-full bg-sky-300/10 blur-3xl" />

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
                      <span>{t("liveLabel")}</span>
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
                      {t("panelTaskLabel")}
                    </div>
                    <div className="mt-2 max-w-[38rem] text-[15px] font-medium leading-[1.65] tracking-[-0.02em] text-white/90 sm:text-[16px]">
                      {t("panelTaskValue")}
                    </div>

                    <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px] uppercase tracking-[0.12em] text-white/38">
                      <div className="inline-flex items-center gap-2">
                        <span>{t("panelQueueLabel")}</span>
                        <span className="text-white/74">{t("panelQueueValue")}</span>
                      </div>

                      <div className="inline-flex items-center gap-2">
                        <span>{t("panelRuntimeLabel")}</span>
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

              <div className="relative px-5 py-5 sm:px-6 sm:py-6">
                {/* ── SVG-hero replaces the three.js scene ──────────────── */}
                <div
                  ref={heroRef}
                  className="relative flex items-center justify-center overflow-hidden rounded-[24px] border border-white/8 bg-[linear-gradient(180deg,rgba(7,10,15,0.5),rgba(3,5,10,0.7))] px-6 py-12 sm:py-14"
                >
                  <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_50%,rgba(255,255,255,0.04),transparent_55%)]" />
                  <StepHero activeIndex={activeIndex} steps={steps} />
                </div>

                <div
                  key={`${activeStep.id}-detail`}
                  aria-live="polite"
                  className="mt-6 animate-in fade-in-0 slide-in-from-bottom-2 rounded-[24px] bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] px-4 py-4 shadow-[0_0_0_1px_rgba(255,255,255,0.06)] duration-500 backdrop-blur-xl sm:px-5"
                >
                  <div className="grid gap-6 xl:grid-cols-[minmax(0,1.08fr)_minmax(0,0.92fr)]">
                    <div>
                      <div className="text-[11px] uppercase tracking-[0.12em] text-white/36">
                        {t("activeFocusLabel")}
                      </div>
                      <div className="mt-3 text-[22px] font-semibold tracking-[-0.045em] text-white/94">
                        {activeStep.title}
                      </div>
                      <p className="mt-2 max-w-[36rem] text-[14px] leading-[1.72] text-white/58">
                        {activeStep.description}
                      </p>

                      <div className="mt-5 border-t border-white/8 pt-4">
                        <div className="text-[11px] uppercase tracking-[0.12em] text-white/34">
                          {t("stageNoteLabel")}
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
                          {t("panelNextLabel")}
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
