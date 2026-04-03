import type { Metadata } from "next";
import { AgentraLanding } from "@/features/landing/components/agentra-landing";

export const metadata: Metadata = {
  title: {
    absolute: "Agentra — AI-Native Task Management",
  },
  description:
    "Open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  openGraph: {
    title: "Agentra — AI-Native Task Management",
    description:
      "Manage your human + agent workforce in one place.",
    url: "/",
  },
  alternates: {
    canonical: "/",
  },
};

export default function LandingPage() {
  return <AgentraLanding />;
}
