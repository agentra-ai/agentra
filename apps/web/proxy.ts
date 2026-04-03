import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function proxy(request: NextRequest) {
  const loggedIn = request.cookies.has("agentra_logged_in");
  if (loggedIn) {
    return NextResponse.redirect(new URL("/issues", request.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/"],
};
