import React from 'react';
import { DownloadStates, downloadStateDescription, downloadStateLabel } from '@/lib/downloads';
import { cn } from '@/lib/utils';

const chipStyles: Record<string, string> = {
  [DownloadStates.downloading]: 'bg-state-downloading/15 text-state-downloading',
  [DownloadStates.available]: 'bg-state-available/15 text-state-available',
  [DownloadStates.waiting]: 'bg-muted text-state-waiting',
  [DownloadStates.failed]: 'bg-state-failed/15 text-state-failed',
  [DownloadStates.expired]: 'bg-muted text-state-expired line-through',
  [DownloadStates.completed]: 'bg-state-completed/15 text-state-completed',
};

// StateChip shows a download state as a small pill.
export const StateChip: React.FC<{ state: string }> = ({ state }) => (
  <span
    title={downloadStateDescription(state)}
    className={cn('w-fit whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] tracking-[0.4px]', chipStyles[state])}
  >
    {downloadStateLabel(state)}
  </span>
);
