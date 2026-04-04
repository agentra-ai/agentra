import type { Metadata, Viewport } from "next";
import { cookies } from "next/headers";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";
import { cn } from "@/lib/utils";
import { AuthInitializer } from "@/features/auth";
import { WSProvider } from "@/features/realtime";
import { ModalRegistry } from "@/features/modals";
import "./globals.css";

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#ffffff" },
    { media: "(prefers-color-scheme: dark)", color: "#05070b" },
  ],
};

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: "Agentra — AI-Native Task Management",
    template: "%s | Agentra",
  },
  description:
    "Open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  icons: {
    icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
    shortcut: ["/favicon.svg"],
  },
  openGraph: {
    type: "website",
    siteName: "Agentra",
    locale: "en_US",
  },
  twitter: {
    card: "summary_large_image",
  },
  alternates: {
    canonical: "/",
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const locale = cookieStore.get("agentra-locale")?.value;
  const lang = locale === "zh" ? "zh" : "en";

  return (
    <html
      lang={lang}
      suppressHydrationWarning
      className={cn("antialiased font-sans h-full")}
    >
      <body className="h-full overflow-hidden">
        <ThemeProvider>
          <AuthInitializer>
            <WSProvider>{children}</WSProvider>
          </AuthInitializer>
          <ModalRegistry />
          <Toaster />
        </ThemeProvider>
      </body>
    </html>
  );
}
