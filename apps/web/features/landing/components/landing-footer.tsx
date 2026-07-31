"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { useRouter, usePathname } from "next/navigation";
import { useTransition } from "react";
import { AgentraIcon } from "@/components/agentra-icon";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/features/auth";
import { headerButtonClassName } from "./shared";

// Compute the year once at module load. Calling new Date() inline in a
// client component caused hydration mismatches across the year
// boundary (server renders 2026 at 23:59:59 UTC, client renders 2027
// at 00:00:00 local). Module-level constants are evaluated once and
// match on both sides unless the page is open across a year boundary.
const currentYear = new Date().getFullYear();

const LOCALE_OPTIONS = [
  { code: "en", label: "EN" },
  { code: "zh-CN", label: "中文" },
] as const;

function setLocaleCookie(locale: "en" | "zh-CN") {
  document.cookie = `agentra-locale=${locale}; path=/; max-age=31536000; samesite=lax`;
}

export function LandingFooter() {
  const t = useTranslations("landing.footer");
  const tHeader = useTranslations("landing.header");
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const [isPending, startTransition] = useTransition();
  const user = useAuthStore((state) => state.user);

  const switchTo = (next: "en" | "zh-CN") => {
    setLocaleCookie(next);
    startTransition(() => {
      router.replace(pathname ?? "/");
    });
  };

  return (
    <footer className="border-t border-white/8 bg-landing-bg-deep text-white">
      <div className="mx-auto max-w-[1320px] px-4 py-6 sm:px-6 sm:py-8 lg:px-8 lg:py-10">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between lg:gap-8">
          <div className="max-w-[32rem]">
            <Link href="/" className="flex items-center gap-3">
              <AgentraIcon className="size-5 text-white" noSpin />
              <span className="text-[18px] font-semibold tracking-[0.04em] lowercase">
                agentra
              </span>
            </Link>
            <p className="mt-4 text-[15px] leading-[1.8] text-white/54">
              {t("tagline")}
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link
              href={user ? "/issues" : "/login"}
              className={headerButtonClassName("solid", "dark")}
            >
              {user ? tHeader("dashboard") : t("cta")}
            </Link>
          </div>
        </div>

        <div className="mt-6 flex flex-col gap-4 border-t border-white/8 pt-5 sm:mt-8 sm:flex-row sm:items-center sm:justify-between sm:pt-6">
          <nav className="flex flex-wrap items-center gap-5 text-[14px] text-white/58">
            <Link href="/about" className="transition-colors hover:text-white">
              {t("links.about")}
            </Link>
            <Link
              href="/changelog"
              className="transition-colors hover:text-white"
            >
              {t("links.changelog")}
            </Link>
          </nav>

          <div className="flex items-center gap-5">
            <p className="text-[13px] text-white/34">
              {t("copyright", { year: currentYear })}
            </p>
            <div className="flex items-center rounded-full border border-white/10 bg-white/[0.04] p-1">
              {LOCALE_OPTIONS.map((opt) => (
                <button
                  key={opt.code}
                  type="button"
                  onClick={() => switchTo(opt.code)}
                  disabled={isPending}
                  className={cn(
                    "rounded-full px-2.5 py-1 text-[12px] font-medium transition-colors disabled:opacity-50",
                    opt.code === locale
                      ? "bg-white text-landing-bg-deep"
                      : "text-white/42 hover:text-white/72",
                  )}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </footer>
  );
}
