import { NextIntlClientProvider } from "next-intl";
import { getLocale, getTranslations } from "next-intl/server";

export default async function LocaleLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const locale = await getLocale();
  const t = await getTranslations();

  return (
    <NextIntlClientProvider locale={locale} messages={null}>
      {children}
    </NextIntlClientProvider>
  );
}
