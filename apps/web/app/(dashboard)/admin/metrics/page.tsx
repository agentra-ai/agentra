"use client";

import { useEffect, useState } from "react";
import { api } from "@/shared/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

type MetricRow = {
  provider: string;
  total: number;
  successes: number;
  success_rate_pct: string;
  median_duration_ms: number;
  total_cost_usd: number;
};

export default function AdminMetricsPage() {
  const [rows, setRows] = useState<MetricRow[]>([]);
  const [days, setDays] = useState(30);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    api
      .getMetricSummary(days)
      .then((res: any) => setRows(res.providers ?? []))
      .catch((e: any) => setErr(e.message));
  }, [days]);

  if (err) return <div className="p-6 text-destructive">Failed: {err}</div>;

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center gap-4">
        <h1 className="text-2xl font-semibold">Agent Metrics</h1>
        <div className="flex gap-2">
          {[7, 30, 90].map((d) => (
            <Badge key={d} variant={days === d ? "default" : "outline"} className="cursor-pointer"
              onClick={() => setDays(d)}>
              {d}d
            </Badge>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {rows.map((r) => (
          <Card key={r.provider}>
            <CardHeader className="pb-2">
              <CardTitle className="text-base capitalize">{r.provider || "unknown"}</CardTitle>
            </CardHeader>
            <CardContent className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <div className="text-muted-foreground">Total tasks</div>
                <div className="text-lg font-semibold">{r.total}</div>
              </div>
              <div>
                <div className="text-muted-foreground">Success</div>
                <div className="text-lg font-semibold">{r.success_rate_pct}%</div>
              </div>
              <div>
                <div className="text-muted-foreground">Median time</div>
                <div className="font-medium">{(r.median_duration_ms / 1000).toFixed(1)}s</div>
              </div>
              <div>
                <div className="text-muted-foreground">Cost</div>
                <div className="font-medium">${Number(r.total_cost_usd).toFixed(4)}</div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {rows.length === 0 && (
        <div className="rounded-md border p-8 text-center text-muted-foreground">
          No agent metrics recorded yet. Assign an issue to an agent and let it complete to see data here.
        </div>
      )}
    </div>
  );
}
