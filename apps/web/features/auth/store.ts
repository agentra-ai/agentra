"use client";

import { create } from "zustand";
import type { User } from "@/shared/types";
import { api } from "@/shared/api";
import type { SendCodeResponse } from "@/shared/api";
import { setLoggedInCookie, clearLoggedInCookie } from "./auth-cookie";

interface AuthState {
  user: User | null;
  isLoading: boolean;

  initialize: () => Promise<void>;
  sendCode: (email: string) => Promise<SendCodeResponse>;
  verifyCode: (email: string, code: string) => Promise<User>;
  logout: () => void;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,

  initialize: async () => {
    const token = localStorage.getItem("agentra_token");
    if (!token) {
      set({ isLoading: false });
      return;
    }

    api.setToken(token);

    try {
      const user = await api.getMe();
      set({ user, isLoading: false });
    } catch {
      api.setToken(null);
      api.setWorkspaceId(null);
      localStorage.removeItem("agentra_token");
      localStorage.removeItem("agentra_workspace_id");
      set({ user: null, isLoading: false });
    }
  },

  sendCode: async (email: string) => {
    return api.sendCode(email);
  },

  verifyCode: async (email: string, code: string) => {
    const { token, user } = await api.verifyCode(email, code);
    localStorage.setItem("agentra_token", token);
    api.setToken(token);
    setLoggedInCookie();
    set({ user });
    return user;
  },

  logout: () => {
    localStorage.removeItem("agentra_token");
    localStorage.removeItem("agentra_workspace_id");
    api.setToken(null);
    api.setWorkspaceId(null);
    clearLoggedInCookie();
    set({ user: null });
  },

  setUser: (user: User) => {
    set({ user });
  },
}));
