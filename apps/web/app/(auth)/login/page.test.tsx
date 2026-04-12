import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { messages } from "@/i18n";
import type { Message } from "@/i18n";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(),
}));

// Mock auth store
const mockSendCode = vi.fn();
const mockVerifyCode = vi.fn();
vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: any) => any) =>
    selector({
      sendCode: mockSendCode,
      verifyCode: mockVerifyCode,
    }),
}));

// Mock workspace store
const mockHydrateWorkspace = vi.fn();
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (selector: (s: any) => any) =>
    selector({
      hydrateWorkspace: mockHydrateWorkspace,
    }),
}));

// Mock api
vi.mock("@/shared/api", () => ({
  api: {
    listWorkspaces: vi.fn().mockResolvedValue([]),
    verifyCode: vi.fn(),
    setToken: vi.fn(),
    getMe: vi.fn(),
  },
}));

import LoginPage from "./page";

const renderWithI18n = (component: React.ReactElement) => {
  return render(
    <NextIntlClientProvider locale="en" messages={messages.en as Message}>
      {component}
    </NextIntlClientProvider>
  );
};

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders login form with email input and continue button", () => {
    renderWithI18n(<LoginPage />);

    expect(screen.getByText("Agentra")).toBeInTheDocument();
    expect(screen.getByText("Turn coding agents into real teammates")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Sign In" })
    ).toBeInTheDocument();
  });

  it("does not call sendCode when email is empty", async () => {
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Sign In" }));
    expect(mockSendCode).not.toHaveBeenCalled();
  });

  it("calls sendCode with email on submit", async () => {
    mockSendCode.mockResolvedValueOnce({ message: "Verification code sent" });
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@agentra.ai");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(mockSendCode).toHaveBeenCalledWith("test@agentra.ai");
    });
  });

  it("shows 'Signing in...' while submitting", async () => {
    mockSendCode.mockReturnValueOnce(new Promise(() => {}));
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@agentra.ai");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(screen.getByText("Signing in...")).toBeInTheDocument();
    });
  });

  it("shows verification code step after sending code", async () => {
    mockSendCode.mockResolvedValueOnce({ message: "Verification code sent" });
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@agentra.ai");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(screen.getByText("Check your email")).toBeInTheDocument();
    });
  });

  it("shows error when sendCode fails", async () => {
    mockSendCode.mockRejectedValueOnce(new Error("Network error"));
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@agentra.ai");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });

  it("shows the development code when email delivery is not configured", async () => {
    mockSendCode.mockResolvedValueOnce({
      message: "Verification code sent",
      dev_code: "123456",
    });
    const user = userEvent.setup();
    renderWithI18n(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@agentra.ai");
    await user.click(screen.getByRole("button", { name: "Sign In" }));

    await waitFor(() => {
      expect(screen.getByText("Development Code")).toBeInTheDocument();
      expect(screen.getByText("123456")).toBeInTheDocument();
    });
  });
});
