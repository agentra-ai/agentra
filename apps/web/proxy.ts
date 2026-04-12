import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const LOCALES = ["en", "zh-CN", "zh"];
const DEFAULT_LOCALE = "en";
const LOCALE_COOKIE = "agentra-locale";

export function proxy(request: NextRequest) {
  const response = NextResponse.next();

  // Check if locale cookie already exists
  const localeCookie = request.cookies.get(LOCALE_COOKIE)?.value;

  if (!localeCookie) {
    // Detect locale from Accept-Language header
    const acceptLang = request.headers.get("accept-language") ?? "";
    let detectedLocale = DEFAULT_LOCALE;

    if (acceptLang.includes("zh")) {
      detectedLocale = "zh-CN";
    }

    // Set the cookie
    response.cookies.set(LOCALE_COOKIE, detectedLocale, {
      path: "/",
      maxAge: 60 * 60 * 24 * 365,
      sameSite: "lax",
    });
  }

  return response;
}

export const config = {
  matcher: [
    // Apply to all app routes
    "/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)",
  ],
};
