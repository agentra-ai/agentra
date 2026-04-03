import { cookies, headers } from "next/headers";
import { LocaleProvider } from "@/features/landing/i18n";
import type { Locale } from "@/features/landing/i18n";

const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      name: "Agentra",
      url: "https://www.agentra.ai",
      sameAs: ["https://github.com/agentra-ai/agentra"],
    },
    {
      "@type": "SoftwareApplication",
      name: "Agentra",
      applicationCategory: "ProjectManagement",
      operatingSystem: "Web",
      description:
        "AI-native task management platform that turns coding agents into real teammates.",
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
    },
  ],
};

async function getInitialLocale(): Promise<Locale> {
  // 1. User's explicit preference (cookie set when they switch language)
  const cookieStore = await cookies();
  const stored = cookieStore.get("agentra-locale")?.value;
  if (stored === "en" || stored === "zh") return stored;

  // 2. Detect from Accept-Language header
  const headersList = await headers();
  const acceptLang = headersList.get("accept-language") ?? "";
  if (acceptLang.includes("zh")) return "zh";

  return "en";
}

export default async function LandingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const initialLocale = await getInitialLocale();

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <div className="h-full overflow-x-hidden overflow-y-auto bg-white">
        <LocaleProvider initialLocale={initialLocale}>{children}</LocaleProvider>
      </div>
    </>
  );
}
