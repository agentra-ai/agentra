"use client";

import { Eye, Layers3, Route } from "lucide-react";
import { useLocale } from "../i18n";

const icons = [Route, Eye, Layers3];

export function WhyAgentraSection() {
  const { t } = useLocale();

  return (
    <section className="bg-[#f5f8fc] text-[#0a0d12]">
      <div className="mx-auto max-w-[1320px] px-4 py-24 sm:px-6 sm:py-28 lg:px-8 lg:py-32">
        <div className="max-w-[760px]">
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[#0a0d12]/42">
            {t.valueProps.label}
          </p>
          <h2 className="mt-4 font-[family-name:var(--font-serif)] text-[2.6rem] leading-[1.05] tracking-[-0.03em] sm:text-[3.4rem] lg:text-[4.2rem]">
            {t.valueProps.headline}
          </h2>
          <p className="mt-5 max-w-[680px] text-[15px] leading-7 text-[#0a0d12]/62 sm:text-[16px]">
            {t.valueProps.description}
          </p>
        </div>

        <div className="mt-14 grid gap-5 lg:grid-cols-3">
          {t.valueProps.items.map((item, index) => {
            const Icon = icons[index] ?? Route;
            return (
              <div
                key={item.title}
                className="relative overflow-hidden rounded-[28px] border border-[#0a0d12]/8 bg-white p-7 shadow-[0_16px_48px_rgba(15,23,42,0.06)] sm:p-8"
              >
                <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-[#dbeafe] via-[#93c5fd] to-[#dbeafe]" />
                <div className="grid size-11 place-items-center rounded-2xl bg-[#edf5ff] text-[#295ca8]">
                  <Icon className="size-5" />
                </div>
                <h3 className="mt-6 text-[20px] font-semibold tracking-[-0.02em] text-[#0a0d12]">
                  {item.title}
                </h3>
                <p className="mt-3 text-[14px] leading-[1.8] text-[#0a0d12]/60 sm:text-[15px]">
                  {item.description}
                </p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
