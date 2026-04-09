"use client";

import { LandingHeader } from "./landing-header";
import { LandingFooter } from "./landing-footer";
import { LandingTheater } from "./landing-theater";
import { LandingValueProps } from "./landing-value-props";

export function AgentraLanding() {
  return (
    <>
      <div className="relative">
        <LandingHeader />
        <LandingTheater />
      </div>
      <LandingValueProps />
      <LandingFooter />
    </>
  );
}
