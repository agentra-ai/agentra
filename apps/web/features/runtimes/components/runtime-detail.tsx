"use client";

import { useFormatter, useTranslations } from "next-intl";
import type { AgentRuntime } from "@/shared/types";
import { formatLastSeen } from "../utils";
import { RuntimeModeIcon, StatusBadge, InfoField } from "./shared";
import { PingSection } from "./ping-section";
import { UpdateSection } from "./update-section";
import { UsageSection } from "./usage-section";
import { CapabilitySection } from "./capability-section";
import { useRuntimeStore } from "../store";

function getCliVersion(metadata: Record<string, unknown>): string | null {
  if (
    metadata &&
    typeof metadata.cli_version === "string" &&
    metadata.cli_version
  ) {
    return metadata.cli_version;
  }
  return null;
}

export function RuntimeDetail({ runtime }: { runtime: AgentRuntime }) {
  const t = useTranslations("runtimes");
  const f = useFormatter();
  const cliVersion =
    runtime.runtime_mode === "local" ? getCliVersion(runtime.metadata) : null;
  const fetchRuntimes = useRuntimeStore((s) => s.fetchRuntimes);

  const handleUpdateComplete = () => {
    // Refetch runtimes after daemon restarts with new version.
    // The daemon:register event should handle this, but being explicit
    // ensures the UI reflects the new version promptly.
    fetchRuntimes();
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
        <div className="flex min-w-0 items-center gap-2">
          <div
            className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
              runtime.status === "online" ? "bg-success/10" : "bg-muted"
            }`}
          >
            <RuntimeModeIcon mode={runtime.runtime_mode} />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-semibold truncate">{runtime.name}</h2>
          </div>
        </div>
        <StatusBadge status={runtime.status} />
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* Info grid */}
        <div className="grid grid-cols-2 gap-4">
          <InfoField label={t("detail.runtimeMode")} value={runtime.runtime_mode} />
          <InfoField label={t("detail.provider")} value={runtime.provider} />
          <InfoField label={t("detail.status")} value={runtime.status} />
          <InfoField
            label={t("detail.lastSeen")}
            value={formatLastSeen(runtime.last_seen_at)}
          />
          {runtime.device_info && (
            <InfoField label={t("detail.device")} value={runtime.device_info} />
          )}
          {runtime.daemon_id && (
            <InfoField label={t("detail.daemonId")} value={runtime.daemon_id} mono />
          )}
        </div>

        {/* CLI Version & Update */}
        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-3">
              {t("detail.cliVersion")}
            </h3>
            <UpdateSection
              runtimeId={runtime.id}
              currentVersion={cliVersion}
              isOnline={runtime.status === "online"}
              onUpdateComplete={handleUpdateComplete}
            />
          </div>
        )}

        {runtime.runtime_mode === "local" && (
          <div>
            <h3 className="mb-3 text-xs font-medium text-muted-foreground">
              {t("detail.capabilities")}
            </h3>
            <CapabilitySection adapter={runtime.adapter} />
          </div>
        )}

        {/* Connection Test */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            {t("detail.connectionTest")}
          </h3>
          <PingSection runtimeId={runtime.id} />
        </div>

        {/* Usage */}
        <div>
          <h3 className="text-xs font-medium text-muted-foreground mb-3">
            {t("detail.tokenUsage")}
          </h3>
          <UsageSection runtimeId={runtime.id} />
        </div>

        {/* Metadata */}
        {runtime.metadata && Object.keys(runtime.metadata).length > 0 && (
          <div>
            <h3 className="text-xs font-medium text-muted-foreground mb-2">
              {t("detail.metadata")}
            </h3>
            <div className="rounded-lg border bg-muted/30 p-3">
              <pre className="text-xs font-mono whitespace-pre-wrap break-all">
                {JSON.stringify(runtime.metadata, null, 2)}
              </pre>
            </div>
          </div>
        )}

        {/* Timestamps */}
        <div className="grid grid-cols-2 gap-4 border-t pt-4">
          <InfoField
            label={t("detail.created")}
            value={f.dateTime(new Date(runtime.created_at), { dateStyle: "short" })}
          />
          <InfoField
            label={t("detail.updated")}
            value={f.dateTime(new Date(runtime.updated_at), { dateStyle: "short" })}
          />
        </div>
      </div>
    </div>
  );
}
