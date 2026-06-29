import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { LandingHeader } from "./landing-header";
import { LandingFooter } from "./landing-footer";

export async function AboutPageClient() {
  const t = await getTranslations("landing");
  const title = t("about.title");
  const paragraphs = t.raw("about.paragraphs") as string[];
  const cta = t("footer.cta");

  return (
    <>
      <LandingHeader variant="light" />
      <main className="bg-white text-landing-fg">
        <div className="mx-auto max-w-[720px] px-4 py-16 sm:px-6 sm:py-20 lg:py-24">
          <h1 className="font-[family-name:var(--font-serif)] text-[2.6rem] leading-[1.05] tracking-[-0.03em] sm:text-[3.4rem]">
            {title}
          </h1>
          <div className="mt-8 space-y-6 text-[15px] leading-[1.8] text-landing-fg/70 sm:text-[16px]">
            {paragraphs.map((p, i) => (
              <p key={i}>{p}</p>
            ))}
          </div>

          <div className="mt-12">
            <Link
              href="/login"
              className="inline-flex items-center gap-2.5 rounded-[12px] bg-landing-surface px-5 py-3 text-[14px] font-semibold text-white transition-colors hover:bg-landing-surface/88"
            >
              {cta}
            </Link>
          </div>
        </div>
      </main>
      <LandingFooter />
    </>
  );
}
