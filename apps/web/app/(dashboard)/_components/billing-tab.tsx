"use client";

import { useEffect, useState, useCallback } from "react";
import { CreditCard, ExternalLink, Loader2 } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { api } from "@/shared/api";

interface Subscription {
  plan?: string;
  status?: string;
  seats?: number;
  stripe_subscription_id?: string | null;
}

interface Usage {
  seats_used?: number;
  seats_max?: number;
  plan?: string;
}

interface Invoice {
  id?: string;
  stripe_invoice_id?: string;
  amount_cents?: number;
  currency?: string;
  status?: string;
  period_start?: string;
  period_end?: string;
  hosted_invoice_url?: string;
  created_at?: string;
}

function badgeVariant(status: string | undefined): "default" | "outline" | "destructive" | "secondary" | "ghost" {
  switch (status) {
    case "active":
    case "paid":
      return "default";
    case "canceled":
    case "unpaid":
      return "destructive";
    case "open":
    case "draft":
      return "secondary";
    case "void":
      return "ghost";
    default:
      return "outline";
  }
}

function planLabel(plan?: string): string {
  if (!plan) return "Free";
  return plan.charAt(0).toUpperCase() + plan.slice(1);
}

export function BillingTab({ workspaceId }: { workspaceId: string }) {
  const t = useTranslations("settings");
  const tBilling = useTranslations("billing");
  const f = useFormatter();
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [usage, setUsage] = useState<Usage | null>(null);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [loading, setLoading] = useState(true);
  const [subscribing, setSubscribing] = useState(false);
  const [managing, setManaging] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [sub, usg, inv] = await Promise.all([
        api.get<Subscription>(`/api/workspaces/${workspaceId}/billing/subscription`),
        api.get<Usage>(`/api/workspaces/${workspaceId}/billing/usage`),
        api.get<Invoice[]>(`/api/workspaces/${workspaceId}/billing/invoices`),
      ]);
      setSubscription(sub);
      setUsage(usg);
      setInvoices(inv ?? []);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("failedToLoad") ?? "Failed to load billing");
      setSubscription({ plan: "free", status: "inactive" });
    } finally {
      setLoading(false);
    }
  }, [t, workspaceId]);

  useEffect(() => {
    load();
  }, [load]);

  const onSubscribe = async () => {
    setSubscribing(true);
    try {
      const res = await api.post<{ url: string }>(`/api/workspaces/${workspaceId}/billing/checkout`);
      if (res?.url) {
        window.location.href = res.url;
      } else {
        toast.error(tBilling("checkoutFailed"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tBilling("checkoutFailed"));
    } finally {
      setSubscribing(false);
    }
  };

  const onManage = async () => {
    setManaging(true);
    try {
      const res = await api.post<{ url: string }>(`/api/workspaces/${workspaceId}/billing/portal`);
      if (res?.url) {
        window.location.href = res.url;
      } else {
        toast.error(tBilling("portalFailed"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : tBilling("portalFailed"));
    } finally {
      setManaging(false);
    }
  };

  const isActive = subscription?.status === "active";
  const isPro = subscription?.plan === "pro";

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex-row items-center gap-4">
          <div className="flex-1 min-w-0">
            <CardTitle>{tBilling("currentPlan")}</CardTitle>
            <p className="text-sm text-muted-foreground mt-1">
              {tBilling("seatUsage", {
                used: usage?.seats_used ?? 0,
                max: usage?.seats_max ?? 0,
              })}
            </p>
          </div>
          <Badge variant={badgeVariant(subscription?.status)} className="text-sm">
            {isPro ? planLabel(subscription?.plan) : tBilling("freePlan")}
          </Badge>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-3 border-t pt-4">
          {!isPro && (
            <Button onClick={onSubscribe} disabled={subscribing} variant="default">
              {subscribing ? <Loader2 className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
              {tBilling("subscribePro")}
            </Button>
          )}
          {isActive && (
            <Button onClick={onManage} disabled={managing} variant="outline">
              {managing ? <Loader2 className="h-4 w-4 animate-spin" /> : <ExternalLink className="h-4 w-4" />}
              {tBilling("manageBilling")}
            </Button>
          )}
          {isPro && !isActive && (
            <span className="text-sm text-muted-foreground self-center">{tBilling("subscriptionInactive")}</span>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{tBilling("invoices")}</CardTitle>
        </CardHeader>
        <CardContent className="border-t pt-4">
          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : invoices.length === 0 ? (
            <p className="text-sm text-muted-foreground">{tBilling("noInvoices")}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-muted-foreground">
                    <th className="pb-2 font-medium">{tBilling("date")}</th>
                    <th className="pb-2 font-medium">{tBilling("amount")}</th>
                    <th className="pb-2 font-medium">{tBilling("status")}</th>
                    <th className="pb-2 font-medium sr-only">{tBilling("actions")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {invoices.map((inv) => (
                    <tr key={inv.id ?? inv.stripe_invoice_id ?? inv.created_at} className="group">
                      <td className="py-2">
                        {inv.created_at ? f.dateTime(new Date(inv.created_at), { dateStyle: "medium" }) : inv.stripe_invoice_id}
                      </td>
                      <td className="py-2">
                        {typeof inv.amount_cents === "number"
                          ? f.number(inv.amount_cents / 100, { style: "currency", currency: inv.currency ?? "USD" })
                          : "—"}
                      </td>
                      <td className="py-2">
                        <Badge variant={badgeVariant(inv.status)}>
                          {inv.status}
                        </Badge>
                      </td>
                      <td className="py-2 text-right">
                        {inv.hosted_invoice_url && (
                          <a
                            href={inv.hosted_invoice_url}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-9 items-center justify-center rounded-md p-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                          >
                            <ExternalLink className="h-4 w-4" />
                          </a>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
