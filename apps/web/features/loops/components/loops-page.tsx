"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw, GitBranch } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import * as loopsApi from "@/shared/api/loops";
import { useLoops } from "../hooks";
import { useLoopStore } from "../store";
import { LoopListRow } from "./loop-list-row";

export function LoopsPage() {
  const t = useTranslations("loops");
  const loops = useLoops();
  const setLoops = useLoopStore((s) => s.setLoops);
  const [refreshing, setRefreshing] = useState(false);
  const [hasFetched, setHasFetched] = useState(false);

  useEffect(() => {
    if (loops.length > 0) setHasFetched(true);
  }, [loops.length]);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const items = await loopsApi.listLoops();
      setLoops(items);
      setHasFetched(true);
    } catch (err) {
      console.error("failed to refresh loops", err);
    } finally {
      setRefreshing(false);
    }
  }, [setLoops]);

  const isLoading = !hasFetched && loops.length === 0;

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <div className="flex h-12 shrink-0 items-center justify-between border-b bg-background px-6">
        <div className="flex items-center gap-2">
          <GitBranch className="h-4 w-4 text-muted-foreground" />
          <h1 className="text-sm font-medium">{t("title")}</h1>
        </div>
        <Button variant="ghost" size="icon-xs" onClick={handleRefresh} disabled={refreshing}>
          <RefreshCw className={`h-4 w-4 ${refreshing ? "animate-spin" : ""}`} />
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-6xl px-6 py-6">
          {isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : loops.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-16 text-sm text-muted-foreground">
              <GitBranch className="h-8 w-8 text-muted-foreground/50" />
              <p>{t("empty")}</p>
              <p className="text-xs">{t("emptyHint")}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("table.id")}</TableHead>
                  <TableHead>{t("table.issue")}</TableHead>
                  <TableHead>{t("table.status")}</TableHead>
                  <TableHead>{t("table.stage")}</TableHead>
                  <TableHead>{t("table.iteration")}</TableHead>
                  <TableHead>{t("table.pr")}</TableHead>
                  <TableHead>{t("table.created")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loops.map((loop) => (
                  <LoopListRow key={loop.id} loop={loop} />
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>
    </div>
  );
}
