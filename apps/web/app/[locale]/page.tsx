import { LocaleProvider } from "@/features/landing/i18n";
import { AgentraLanding } from "@/features/landing/components/agentra-landing";
import type { Locale } from "@/features/landing/i18n";

export default async function LocaleLandingPage({
  params,
}: {
  params: { locale: string };
}) {
  return (
    <LocaleProvider initialLocale={params.locale as Locale}>
      <AgentraLanding />
    </LocaleProvider>
  );
}
