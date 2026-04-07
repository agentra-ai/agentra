"use client";

import { LandingHeader } from "./landing-header";
import { LandingFooter } from "./landing-footer";
import { LandingTheater } from "./landing-theater";

export function AgentraLanding() {
  return (
    <>
      <div className="relative">
        <LandingHeader />
        <LandingTheater />
      </div>
      <LandingFooter />
    </>
  );
}
