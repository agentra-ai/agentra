"use client";

import { use } from "react";
import { LoopDetailPage } from "@/features/loops/components/loop-detail-page";

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <LoopDetailPage id={id} />;
}
