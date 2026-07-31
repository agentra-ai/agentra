"use client";

import { use } from "react";
import { IssueDetail } from "@/features/issues/components";

export default function IssueDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  return <IssueDetail key={id} issueId={id} />;
}
