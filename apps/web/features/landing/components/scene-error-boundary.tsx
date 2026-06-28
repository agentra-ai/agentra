"use client";

import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback: ReactNode;
}

interface State {
  hasError: boolean;
}

/**
 * Catches any render error from the 3D scene and swaps in a static
 * fallback. Without this a single WebGL exception (e.g. unsupported
 * device, blocked third-party cookies) white-screens the whole landing
 * page. The fallback is passed in so the same gradient used during
 * dynamic-loading is reused.
 */
export class SceneErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error) {
    // Surface the real exception to the browser console — a swallowed
    // 3D error is hard to debug otherwise.
    console.error("LandingProofScene error boundary:", error);
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }
    return this.props.children;
  }
}
