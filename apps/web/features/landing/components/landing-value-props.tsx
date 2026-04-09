"use client";

import { cn } from "@/lib/utils";
import { useLocale } from "../i18n";

export function LandingValueProps() {
  const { t } = useLocale();

  return (
    <section className="relative border-t border-white/8 bg-[#020408] text-white">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.04),transparent_28%),linear-gradient(180deg,rgba(255,255,255,0.02),transparent_24%)]" />

      <div className="relative mx-auto max-w-[1320px] px-4 py-16 sm:px-6 lg:px-8 lg:py-20">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,0.86fr)_minmax(0,1.14fr)] lg:items-start">
          <div className="max-w-[34rem]">
            <div className="text-[11px] font-medium uppercase tracking-[0.2em] text-white/42">
              {t.valueProps.label}
            </div>
            <h2 className="mt-4 max-w-[11ch] text-[2.1rem] font-semibold leading-[0.95] tracking-[-0.06em] text-white sm:text-[2.7rem]">
              {t.valueProps.headline}
            </h2>
            <p className="mt-6 max-w-[34rem] text-[15px] leading-[1.9] text-white/58 sm:text-[16px]">
              {t.valueProps.description}
            </p>
          </div>

          <div className="rounded-[28px] bg-[linear-gradient(180deg,rgba(255,255,255,0.04),rgba(255,255,255,0.015))] shadow-[0_0_0_1px_rgba(255,255,255,0.06)] backdrop-blur-xl">
            <div className="grid md:grid-cols-3">
              {t.valueProps.items.map((item, index) => (
                <article
                  key={item.title}
                  className={cn(
                    "px-5 py-6 md:px-6 md:py-8",
                    index < t.valueProps.items.length - 1
                      ? "border-b border-white/8 md:border-b-0 md:border-r"
                      : "",
                  )}
                >
                  <div className="text-[11px] uppercase tracking-[0.16em] text-white/34">
                    {String(index + 1).padStart(2, "0")}
                  </div>
                  <h3 className="mt-8 max-w-[12ch] text-[1.3rem] font-semibold leading-[1.05] tracking-[-0.04em] text-white/94">
                    {item.title}
                  </h3>
                  <p className="mt-4 text-[14px] leading-[1.75] text-white/56">
                    {item.description}
                  </p>
                </article>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
