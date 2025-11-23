import React, { useEffect, useRef } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useAppStore } from "@/store/useAppStore";

const getLogTypeColor = (type: "info" | "success" | "error") => {
  switch (type) {
    case "info":
      return "bg-blue-500/10 text-blue-500 dark:text-blue-400 border border-blue-500/20";
    case "success":
      return "bg-green-500/10 text-green-500 dark:text-green-400 border border-green-500/20";
    case "error":
      return "bg-destructive/10 text-destructive border border-destructive/20";
  }
};

// Strip timestamp patterns from message body (Go log format: YYYY/MM/DD HH:MM:SS)
const stripTimestamp = (message: string): string => {
  // Pattern: YYYY/MM/DD HH:MM:SS (e.g., "2025/11/23 08:59:51")
  const timestampPattern = /^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}\s+/;
  return message.replace(timestampPattern, "").trim();
};

// Format timestamp to yyyy-mm-dd HH:MM:SS format
const formatTimestamp = (timestamp: string): string => {
  try {
    const date = new Date(timestamp);
    if (isNaN(date.getTime())) {
      return timestamp; // Return original if we can't parse it
    }

    // Format as yyyy-mm-dd HH:MM:SS
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    const hours = String(date.getHours()).padStart(2, "0");
    const mins = String(date.getMinutes()).padStart(2, "0");
    const secs = String(date.getSeconds()).padStart(2, "0");

    return `${year}-${month}-${day} ${hours}:${mins}:${secs}`;
  } catch {
    return timestamp; // Return original if formatting fails
  }
};

export const Activity: React.FC = () => {
  const activityLogs = useAppStore((state) => state.activityLogs);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new logs are added
  useEffect(() => {
    if (scrollContainerRef.current) {
      const viewport = scrollContainerRef.current.querySelector(
        '[data-slot="scroll-area-viewport"]',
      ) as HTMLElement;
      if (viewport) {
        viewport.scrollTop = viewport.scrollHeight;
      }
    }
  }, [activityLogs]);

  return (
    <Card className="md:col-span-2 self-start flex flex-col h-[250px]">
      <CardHeader className="shrink-0">
        <CardTitle>Activity</CardTitle>
        <CardDescription>Real-time download activity log</CardDescription>
      </CardHeader>
      <CardContent className="flex-1 min-h-0">
        <div ref={scrollContainerRef} className="h-full">
          <ScrollArea className="h-full">
            <div className="pr-4">
              {activityLogs.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">
                  No activity yet
                </p>
              ) : (
                activityLogs.map((log) => (
                  <div
                    key={log.id}
                    className={`p-1 text-sm ${getLogTypeColor(log.type)}`}
                  >
                    <div className="flex items-start gap-2">
                      <span className="text-xs text-muted-foreground min-w-32">
                        {formatTimestamp(log.timestamp)}
                      </span>
                      <span className="text-xs flex-1">
                        {stripTimestamp(log.message)}
                      </span>
                    </div>
                  </div>
                ))
              )}
            </div>
          </ScrollArea>
        </div>
      </CardContent>
    </Card>
  );
};
