"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronUp, Terminal } from "lucide-react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { StageIndicator } from "./stage-indicator";
import { useStreamingLogs } from "../hooks/use-streaming-logs";
import { useAgentStage } from "../hooks/use-agent-stage";
import type { AgentStage } from "@/shared/types/events";

interface LiveTerminalProps {
  taskId: string | null;
  agentId?: string;
  defaultExpanded?: boolean;
}

export function LiveTerminal({ taskId, defaultExpanded = true }: LiveTerminalProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const { logLines } = useStreamingLogs(taskId);
  const { stage, lastUpdate } = useAgentStage(taskId);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (scrollRef.current && expanded) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logLines, expanded]);

  return (
    <div className="border rounded-lg overflow-hidden bg-card">
      {/* Header */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-3 py-2 bg-muted hover:bg-muted/80 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Terminal className="h-4 w-4" />
          <span className="font-medium text-sm">Agent Output</span>
          {logLines.length > 0 && (
            <span className="text-xs text-muted-foreground">
              ({logLines.length} lines)
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {stage !== "idle" && (
            <StageIndicator stage={stage as AgentStage} timestamp={lastUpdate ?? undefined} />
          )}
          {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
        </div>
      </button>

      {/* Terminal content */}
      {expanded && (
        <ScrollArea className="h-[300px]" ref={scrollRef}>
          <div className="p-3 font-mono text-xs leading-relaxed">
            {logLines.length === 0 ? (
              <div className="text-muted-foreground italic">Waiting for agent output...</div>
            ) : (
              logLines.map((line, i) => (
                <div key={i} className="text-foreground whitespace-pre-wrap break-all">
                  {line}
                </div>
              ))
            )}
          </div>
        </ScrollArea>
      )}
    </div>
  );
}
