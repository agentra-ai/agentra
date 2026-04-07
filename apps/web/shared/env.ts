const trimUrl = (value?: string) => value?.trim().replace(/\/+$/, "") ?? "";

export function getSiteUrl() {
  return trimUrl(
    process.env.NEXT_PUBLIC_SITE_URL ||
      process.env.AGENTRA_APP_URL ||
      process.env.FRONTEND_ORIGIN,
  );
}

export function getRemoteApiUrl() {
  return trimUrl(process.env.REMOTE_API_URL || process.env.NEXT_PUBLIC_API_URL);
}

export function getCliCallbackHosts() {
  return (process.env.NEXT_PUBLIC_CLI_CALLBACK_HOSTS || "")
    .split(",")
    .map((host) => host.trim())
    .filter(Boolean);
}
