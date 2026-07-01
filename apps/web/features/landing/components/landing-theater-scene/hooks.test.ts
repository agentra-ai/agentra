import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDocumentVisibility } from "./hooks";

describe("useDocumentVisibility", () => {
  it("starts true and toggles to false when visibilitychange fires", () => {
    const { result } = renderHook(() => useDocumentVisibility());
    expect(result.current).toBe(true);
    act(() => {
      Object.defineProperty(document, "hidden", { value: true, configurable: true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(result.current).toBe(false);
  });
});
