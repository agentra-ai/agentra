"use client";

import Link from "next/link";
import {
  Activity,
  Bot,
  CheckCircle2,
  Cpu,
  GitBranch,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { AgentraIcon } from "@/components/agentra-icon";
import { useAuthStore } from "@/features/auth";
import { cn } from "@/lib/utils";
import { useLocale } from "../i18n";
import {
  ClaudeCodeLogo,
  CodexLogo,
  heroButtonClassName,
} from "./shared";

const heroEvents = [
  {
    actor: "Agentra",
    text: "Issue API-142 assigned to Claude",
    tone: "neutral" as const,
  },
  {
    actor: "Claude",
    text: "Claimed task and started migration run",
    tone: "success" as const,
  },
  {
    actor: "Runtime",
    text: "Shanghai daemon healthy · queue latency 1.2s",
    tone: "neutral" as const,
  },
];

const heroSkills = [
  "Deploy preview environment",
  "Patch failing migration",
  "Review PR with checklist",
];

export function LandingHero() {
  const { t } = useLocale();
  const user = useAuthStore((s) => s.user);

  return (
    <div className="relative min-h-full overflow-hidden bg-[#05070b] text-white">
      <LandingBackdrop />

      <main className="relative z-10">
        <section
          id="product"
          className="mx-auto max-w-[1320px] px-4 pb-20 pt-28 sm:px-6 sm:pt-32 lg:px-8 lg:pb-28 lg:pt-36"
        >
          <div className="grid gap-12 lg:grid-cols-[minmax(0,1.02fr)_minmax(420px,0.98fr)] lg:items-center lg:gap-14">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border border-white/12 bg-white/6 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-white/68 backdrop-blur-sm">
                <AgentraIcon className="size-3.5 text-[#c7dcff]" noSpin />
                {t.hero.kicker}
              </div>

              <h1 className="mt-6 max-w-[12ch] font-[family-name:var(--font-serif)] text-[3.4rem] leading-[0.92] tracking-[-0.045em] text-white sm:text-[4.8rem] lg:text-[6rem]">
                {t.hero.headlineLine1}
                <br />
                {t.hero.headlineLine2}
              </h1>

              <p className="mt-6 max-w-[700px] text-[15px] leading-7 text-white/78 sm:text-[17px]">
                {t.hero.subheading}
              </p>

              <div className="mt-8 flex flex-wrap items-center gap-3">
                <Link href={user ? "/issues" : "/login"} className={heroButtonClassName("solid")}>
                  {user ? t.header.dashboard : t.hero.cta}
                </Link>
              </div>

              <div className="mt-8 flex flex-wrap gap-2.5">
                {t.hero.proofChips.map((chip) => (
                  <span
                    key={chip}
                    className="inline-flex items-center rounded-full border border-white/10 bg-white/6 px-3 py-1.5 text-[12px] font-medium text-white/72"
                  >
                    {chip}
                  </span>
                ))}
              </div>

              <div className="mt-10 flex flex-wrap items-center gap-5">
                <span className="text-[14px] text-white/44">{t.hero.worksWith}</span>
                <div className="flex items-center gap-5 text-white/78">
                  <div className="flex items-center gap-2.5">
                    <ClaudeCodeLogo className="size-5" />
                    <span className="text-[14px] font-medium">Claude Code</span>
                  </div>
                  <div className="flex items-center gap-2.5">
                    <CodexLogo className="size-5" />
                    <span className="text-[14px] font-medium">Codex</span>
                  </div>
                </div>
              </div>
            </div>

            <HeroControlPlane />
          </div>
        </section>
      </main>
    </div>
  );
}

function HeroControlPlane() {
  return (
    <div className="relative">
      <div className="absolute -left-8 top-10 h-44 w-44 rounded-full bg-[#7dd3fc]/12 blur-3xl" />
      <div className="absolute -bottom-10 right-0 h-48 w-48 rounded-full bg-[#60a5fa]/14 blur-3xl" />

      <div className="relative overflow-hidden rounded-[28px] border border-white/12 bg-[#08111f]/80 p-4 shadow-[0_24px_80px_rgba(0,0,0,0.42)] backdrop-blur-md sm:p-5">
        <div className="rounded-[22px] border border-white/10 bg-[#091423]/92 p-4 sm:p-5">
          <div className="flex flex-wrap items-center gap-3 border-b border-white/10 pb-4">
            <div className="flex items-center gap-2.5">
              <AgentraIcon className="size-5 text-[#d8e6ff]" noSpin />
              <div>
                <p className="text-[13px] font-semibold text-white">Agentra Control Plane</p>
                <p className="text-[11px] text-white/42">Live issue dispatch and runtime telemetry</p>
              </div>
            </div>
            <div className="ml-auto inline-flex items-center gap-2 rounded-full border border-emerald-400/20 bg-emerald-400/8 px-3 py-1 text-[11px] font-medium text-emerald-200">
              <span className="size-1.5 rounded-full bg-emerald-300" />
              4 agents active
            </div>
          </div>

          <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.08fr)_minmax(220px,0.92fr)]">
            <div className="space-y-4">
              <div className="rounded-[20px] border border-white/10 bg-white/[0.03] p-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-white/42">
                  <GitBranch className="size-3.5" />
                  Task Dispatch
                </div>

                <div className="mt-3 rounded-2xl border border-white/10 bg-[#0d1827] p-4">
                  <div className="flex items-start gap-3">
                    <div className="grid size-10 place-items-center rounded-2xl bg-[#1b314f] text-[#d8e6ff]">
                      <Bot className="size-5" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-[12px] font-semibold text-white/46">API-142</span>
                        <span className="rounded-full border border-sky-300/14 bg-sky-300/10 px-2 py-0.5 text-[10px] font-medium text-sky-100">
                          In progress
                        </span>
                      </div>
                      <p className="mt-1 text-[15px] font-semibold leading-snug text-white">
                        Standardize error responses across 14 handlers
                      </p>
                      <p className="mt-1 text-[12px] leading-5 text-white/48">
                        Assigned to Claude with migration, review, and deploy skills attached.
                      </p>
                    </div>
                  </div>

                  <div className="mt-4 grid gap-2 sm:grid-cols-3">
                    {[
                      { label: "Queued", value: "2", tone: "muted" },
                      { label: "Running", value: "1", tone: "active" },
                      { label: "Blocked", value: "0", tone: "clear" },
                    ].map((item) => (
                      <div
                        key={item.label}
                        className={cn(
                          "rounded-2xl border px-3 py-3",
                          item.tone === "active" && "border-sky-300/16 bg-sky-300/10",
                          item.tone === "clear" && "border-emerald-300/14 bg-emerald-300/8",
                          item.tone === "muted" && "border-white/8 bg-white/[0.03]",
                        )}
                      >
                        <p className="text-[11px] uppercase tracking-[0.14em] text-white/40">{item.label}</p>
                        <p className="mt-1 text-[20px] font-semibold text-white">{item.value}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              <div className="rounded-[20px] border border-white/10 bg-white/[0.03] p-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-white/42">
                  <Activity className="size-3.5" />
                  Execution Feed
                </div>
                <div className="mt-3 space-y-2">
                  {heroEvents.map((event, index) => (
                    <div
                      key={`${event.actor}-${index}`}
                      className="flex items-start gap-3 rounded-2xl border border-white/8 bg-[#0d1827] px-3 py-3"
                    >
                      <span
                        className={cn(
                          "mt-1 size-2 rounded-full",
                          event.tone === "success" ? "bg-emerald-300" : "bg-sky-300",
                        )}
                      />
                      <div className="min-w-0 flex-1">
                        <p className="text-[12px] font-medium text-white">{event.actor}</p>
                        <p className="mt-0.5 text-[12px] leading-5 text-white/50">{event.text}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <div className="space-y-4">
              <div className="rounded-[20px] border border-white/10 bg-white/[0.03] p-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-white/42">
                  <Cpu className="size-3.5" />
                  Runtime Fleet
                </div>
                <div className="mt-3 space-y-2.5">
                  {[
                    { name: "Shanghai Mac Studio", status: "Healthy", load: "63%" },
                    { name: "AWS c7g build pool", status: "Healthy", load: "41%" },
                    { name: "Review sandbox", status: "Idle", load: "12%" },
                  ].map((runtime) => (
                    <div
                      key={runtime.name}
                      className="rounded-2xl border border-white/8 bg-[#0d1827] px-3 py-3"
                    >
                      <div className="flex items-center gap-2">
                        <span className="size-2 rounded-full bg-emerald-300" />
                        <p className="min-w-0 flex-1 truncate text-[12px] font-medium text-white">
                          {runtime.name}
                        </p>
                      </div>
                      <div className="mt-2 flex items-center justify-between text-[11px] text-white/44">
                        <span>{runtime.status}</span>
                        <span>{runtime.load}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-[20px] border border-white/10 bg-white/[0.03] p-4">
                <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.14em] text-white/42">
                  <Sparkles className="size-3.5" />
                  Shared Skills
                </div>
                <div className="mt-3 space-y-2">
                  {heroSkills.map((skill) => (
                    <div
                      key={skill}
                      className="flex items-center gap-2 rounded-2xl border border-white/8 bg-[#0d1827] px-3 py-3"
                    >
                      <CheckCircle2 className="size-4 shrink-0 text-sky-200" />
                      <span className="text-[12px] text-white/72">{skill}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-[20px] border border-sky-300/14 bg-sky-300/8 p-4">
                <div className="flex items-start gap-3">
                  <ShieldCheck className="mt-0.5 size-4 shrink-0 text-sky-100" />
                  <div>
                    <p className="text-[12px] font-semibold text-white">Open and self-hostable</p>
                    <p className="mt-1 text-[12px] leading-5 text-white/56">
                      Keep code execution on your own machines or cloud while Agentra coordinates the work.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function LandingBackdrop() {
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(125,211,252,0.16),transparent_28%),radial-gradient(circle_at_bottom_left,rgba(96,165,250,0.14),transparent_34%),linear-gradient(180deg,rgba(6,10,16,0.08),rgba(6,10,16,0.28))]" />
      <div className="absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.04)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.04)_1px,transparent_1px)] bg-[size:72px_72px] opacity-25" />
      <div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-b from-transparent to-[#05070b]" />
    </div>
  );
}
