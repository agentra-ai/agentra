import createMiddleware from "next-intl/middleware";
import { locales, defaultLocale } from "./i18n/request";

export default createMiddleware({
  locales,
  defaultLocale,
  localePrefix: "as-needed",
  localeCookie: {
    name: "agentra-locale",
  },
});

export const config = {
  matcher: [
    // Match all pathnames except for
    // - API routes
    // - Static files
    // - Favicon
    // - Root page (landing page handles its own locale)
    "/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)",
  ],
};
