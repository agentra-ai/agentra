"use client";

import Link from "next/link";
import { ArrowRight, CheckCircle2, Sparkles } from "lucide-react";
import { useAuthStore } from "@/features/auth";
import { useLocale } from "../i18n";
import { heroButtonClassName } from "./shared";

export function HowItWorksSection() {
  const { t } = useLocale();
  const user = useAuthStore((s) => s.user);

  return (
    <section id="how-it-works" className="bg-[#05070b] text-white">
      <div className="mx-auto max-w-[1320px] px-4 py-24 sm:px-6 sm:py-32 lg:px-8 lg:py-36">
        <div className="max-w-[760px]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-white/40">
            {t.howItWorks.label}
          </p>
          <h2 className="mt-4 font-[family-name:var(--font-serif)] text-[2.6rem] leading-[1.05] tracking-[-0.03em] sm:text-[3.4rem] lg:text-[4.2rem]">
            {t.howItWorks.headlineMain}
            <br />
            <span className="text-white/40">{t.howItWorks.headlineFaded}</span>
          </h2>
        </div>

        <div className="mt-14 overflow-hidden rounded-[28px] border border-white/10 bg-white/[0.03] p-5 sm:p-6 lg:p-8">
          <div className="grid gap-4 lg:grid-cols-4">
            {t.howItWorks.steps.map((step, index) => (
              <div key={step.title} className="relative">
                <div className="rounded-[24px] border border-white/10 bg-[#08111d] p-6">
                  <div className="flex items-center justify-between">
                    <div className="grid size-10 place-items-center rounded-full border border-white/12 bg-white/6 text-[13px] font-semibold text-white/82">
                      {String(index + 1).padStart(2, "0")}
                    </div>
                    {index === t.howItWorks.steps.length - 1 ? (
                      <Sparkles className="size-4 text-sky-200" />
                    ) : (
                      <CheckCircle2 className="size-4 text-emerald-300" />
                    )}
                  </div>

                  <h3 className="mt-5 text-[18px] font-semibold leading-snug text-white">
                    {step.title}
                  </h3>
                  <p className="mt-3 text-[14px] leading-[1.8] text-white/54 sm:text-[15px]">
                    {step.description}
                  </p>
                </div>

                {index < t.howItWorks.steps.length - 1 && (
                  <div className="hidden lg:block">
                    <div className="absolute right-[-22px] top-1/2 z-10 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full border border-white/10 bg-[#0a1422] text-white/50">
                      <ArrowRight className="size-4" />
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>

        <div className="mt-10 flex flex-wrap items-center gap-4">
          <Link href={user ? "/issues" : "/login"} className={heroButtonClassName("solid")}>
            {user ? t.header.dashboard : t.howItWorks.cta}
          </Link>
        </div>
      </div>
    </section>
  );
}
