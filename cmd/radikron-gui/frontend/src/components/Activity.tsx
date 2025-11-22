import React, { useEffect, useRef } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useAppStore } from '@/store/useAppStore';

const getLogTypeColor = (type: 'info' | 'success' | 'error') => {
  switch (type) {
    case 'info':
      return 'bg-blue-500/10 text-blue-500 dark:text-blue-400 border border-blue-500/20';
    case 'success':
      return 'bg-green-500/10 text-green-500 dark:text-green-400 border border-green-500/20';
    case 'error':
      return 'bg-destructive/10 text-destructive border border-destructive/20';
  }
};

export const Activity: React.FC = () => {
  const activityLogs = useAppStore((state) => state.activityLogs);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new logs are added
  useEffect(() => {
    if (scrollContainerRef.current) {
      const viewport = scrollContainerRef.current.querySelector('[data-slot="scroll-area-viewport"]') as HTMLElement;
      if (viewport) {
        viewport.scrollTop = viewport.scrollHeight;
      }
    }
  }, [activityLogs]);

  return (
    <Card className="md:col-span-2 self-start flex flex-col h-[250px]">
      <CardHeader className="flex-shrink-0">
        <CardTitle>Activity</CardTitle>
        <CardDescription>Real-time download activity log</CardDescription>
      </CardHeader>
      <CardContent className="flex-1 min-h-0">
        <div ref={scrollContainerRef} className="h-full">
          <ScrollArea className="h-full">
            <div className="space-y-2 pr-4">
            {activityLogs.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-8">
                No activity yet
              </p>
            ) : (
              activityLogs.map((log) => (
                <div
                  key={log.id}
                  className={`p-3 rounded-md border text-sm ${getLogTypeColor(log.type)}`}
                >
                  <div className="flex items-start gap-2">
                    <span className="text-xs text-muted-foreground min-w-[80px]">
                      {log.timestamp}
                    </span>
                    <span className="flex-1">{log.message}</span>
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

