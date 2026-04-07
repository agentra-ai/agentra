"use client";

import Link from "next/link";
import { AgentraIcon } from "@/components/agentra-icon";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/features/auth";
import { localeLabels, locales, useLocale } from "../i18n";
import { GitHubMark, githubUrl, headerButtonClassName } from "./shared";

export function LandingFooter() {
  const { t, locale, setLocale } = useLocale();
  const user = useAuthStore((state) => state.user);

  return (
    <footer className="border-t border-white/8 bg-[#05070b] text-white">
      <div className="mx-auto max-w-[1320px] px-4 py-8 sm:px-6 lg:px-8 lg:py-10">
        <div className="flex flex-col gap-8 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-[32rem]">
            <Link href="/" className="flex items-center gap-3">
              <AgentraIcon className="size-5 text-white" noSpin />
              <span className="text-[18px] font-semibold tracking-[0.04em] lowercase">
                agentra
              </span>
            </Link>
            <p className="mt-4 text-[15px] leading-[1.8] text-white/54">
              {t.footer.tagline}
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link
              href={user ? "/issues" : "/login"}
              className={headerButtonClassName("solid", "dark")}
            >
              {user ? t.header.dashboard : t.footer.cta}
            </Link>
            <Link
              href={githubUrl}
              target="_blank"
              rel="noreferrer"
              className={headerButtonClassName("ghost", "dark")}
            >
              <GitHubMark className="size-4" />
              {t.footer.links.github}
            </Link>
          </div>
        </div>

        <div className="mt-8 flex flex-col gap-4 border-t border-white/8 pt-6 sm:flex-row sm:items-center sm:justify-between">
          <nav className="flex flex-wrap items-center gap-5 text-[14px] text-white/58">
            <Link href="/about" className="transition-colors hover:text-white">
              {t.footer.links.about}
            </Link>
            <Link
              href="/changelog"
              className="transition-colors hover:text-white"
            >
              {t.footer.links.changelog}
            </Link>
            <Link
              href={githubUrl}
              target="_blank"
              rel="noreferrer"
              className="transition-colors hover:text-white"
            >
              {t.footer.links.github}
            </Link>
          </nav>

          <div className="flex items-center gap-5">
            <p className="text-[13px] text-white/34">
              {t.footer.copyright.replace(
                "{year}",
                String(new Date().getFullYear()),
              )}
            </p>
            <div className="flex items-center rounded-full border border-white/10 bg-white/[0.04] p-1">
              {locales.map((value) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setLocale(value)}
                  className={cn(
                    "rounded-full px-2.5 py-1 text-[12px] font-medium transition-colors",
                    value === locale
                      ? "bg-white text-[#05070b]"
                      : "text-white/42 hover:text-white/72",
                  )}
                >
                  {localeLabels[value]}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </footer>
  );
}
