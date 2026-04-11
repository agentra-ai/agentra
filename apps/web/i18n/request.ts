import { getRequestConfig } from "next-intl/server";
import { cookies } from "next/headers";

export const locales = ["en", "zh-CN"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "en";

export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  // Support both 'zh' (legacy) and 'zh-CN' formats
  let locale: Locale = defaultLocale;
  const cookieLocale = cookieStore.get("agentra-locale")?.value;

  if (cookieLocale === "zh-CN" || cookieLocale === "zh") {
    locale = "zh-CN";
  }

  return {
    locale,
    messages: (await import(`../messages/${locale}.json`)).default,
  };
});
