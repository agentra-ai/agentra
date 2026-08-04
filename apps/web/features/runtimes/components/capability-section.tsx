"use client";

import { useTranslations } from "next-intl";
import type {
  RuntimeAdapterContract,
  RuntimeCapabilityLevel,
} from "@/shared/types";
import {
  formatCapabilityName,
  RUNTIME_CAPABILITIES,
  summarizeCapabilities,
} from "../capabilities";

const LEVEL_STYLES: Record<RuntimeCapabilityLevel, string> = {
  native: "bg-success/10 text-success",
  adapter: "bg-primary/10 text-primary",
  unsupported: "bg-muted text-muted-foreground",
};

export function CapabilitySection({
  adapter,
}: {
  adapter?: RuntimeAdapterContract;
}) {
  const t = useTranslations("runtimes.capabilities");

  if (!adapter) {
    return (
      <div className="rounded-lg border bg-muted/30 p-3 text-xs text-muted-foreground">
        {t("unavailable")}
      </div>
    );
  }

  const summary = summarizeCapabilities(adapter);

  return (
    <div className="space-y-3 rounded-lg border p-3">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="text-muted-foreground">
          {t("summary", summary)}
        </span>
        <span className="font-mono text-muted-foreground">
          {adapter.version} · {adapter.transport}
        </span>
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        {RUNTIME_CAPABILITIES.map((capability) => {
          const support = adapter.capabilities[capability];
          const level = support?.level ?? "unsupported";
          return (
            <div
              key={capability}
              className="flex min-w-0 items-start justify-between gap-2 rounded-md bg-muted/30 px-2.5 py-2"
              title={support?.detail}
            >
              <span className="truncate text-xs font-medium">
                {formatCapabilityName(capability)}
              </span>
              <span
                className={`shrink-0 rounded-full px-2 py-0.5 text-[10px] font-medium ${LEVEL_STYLES[level]}`}
              >
                {t(level)}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
