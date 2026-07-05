"use client";

import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";

export default function AgentDetailPage() {
  const t = useTranslations("agents");
  const params = useParams<{ id: string }>();

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold">{t("title")} — {params.id}</h1>
      <p className="mt-2 text-muted-foreground">{t("instructions")}</p>
    </div>
  );
}
