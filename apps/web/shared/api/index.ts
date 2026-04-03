import { createLogger } from "@/shared/logger";
import { ApiClient } from "./client";

export { ApiClient } from "./client";
export type { LoginResponse } from "./client";
export type { SendCodeResponse } from "./client";
export { WSClient } from "./ws-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "";

export const api = new ApiClient(API_BASE_URL, { logger: createLogger("api") });

// Initialize token from localStorage on load
if (typeof window !== "undefined") {
  const token = localStorage.getItem("agentra_token");
  if (token) {
    api.setToken(token);
  }
  const wsId = localStorage.getItem("agentra_workspace_id");
  if (wsId) {
    api.setWorkspaceId(wsId);
  }

}
