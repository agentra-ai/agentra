"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Loader2, Sparkles } from "lucide-react";
import { useTaskGraph } from "../hooks/useTaskGraph";

interface AutoDecomposeButtonProps {
  issueId: string;
}

export function AutoDecomposeButton({ issueId }: AutoDecomposeButtonProps) {
  const t = useTranslations("taskGraph");
  const { decomposeGraph, isLoading } = useTaskGraph();
  const [decomposing, setDecomposing] = useState(false);

  const handleDecompose = async (maxNodes: number = 10) => {
    setDecomposing(true);
    try {
      await decomposeGraph(issueId, { maxNodes });
    } finally {
      setDecomposing(false);
    }
  };

  if (isLoading || decomposing) {
    return (
      <Button variant="outline" size="sm" disabled>
        <Loader2 className="h-4 w-4 mr-2 animate-spin" />
        {t("autoDecompose.decomposing")}
      </Button>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" size="sm">
            <Sparkles className="h-4 w-4 mr-2" />
            {t("autoDecompose.button")}
          </Button>
        }
      />
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => handleDecompose(5)}>
          {t("autoDecompose.quick")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleDecompose(10)}>
          {t("autoDecompose.balanced")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleDecompose(15)}>
          {t("autoDecompose.detailed")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => handleDecompose(20)}>
          {t("autoDecompose.comprehensive")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}