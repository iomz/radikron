import React, { useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import { downloadAction, type DownloadAction } from '@/lib/downloads';
import { cn } from '@/lib/utils';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const labels: Record<DownloadAction, { button: string; done: string }> = {
  cancel: { button: 'Cancel', done: 'Canceled' },
  skip: { button: 'Skip', done: 'Skipped' },
  undo: { button: 'Undo skip', done: 'Radikron will download' },
};

// DownloadActionButton cancels a download in progress, skips one that has not
// started, or undoes a skip. Canceling and skipping can be undone from the
// toast that confirms them.
export const DownloadActionButton: React.FC<{ item: DownloadItem; className?: string; onDone?: () => void }> = ({ item, className, onDone }) => {
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [busy, setBusy] = useState(false);
  const action = downloadAction(item);
  if (!action) return null;

  const undo = async () => {
    try {
      await App.UndoSkip(item.stationId, item.start);
      await loadDownloads();
    } catch (err) {
      toast.error(`Couldn't undo it: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const run = async () => {
    setBusy(true);
    try {
      if (action === 'undo') {
        await App.UndoSkip(item.stationId, item.start);
      } else {
        await App.SkipDownload(item.stationId, item.start);
      }
      const message = `${labels[action].done} ${item.title}`;
      addActivityLog('success', message);
      toast.success(message, action === 'undo' ? undefined : { action: { label: 'Undo', onClick: () => void undo() } });
      await loadDownloads();
      onDone?.();
    } catch (err) {
      toast.error(`Couldn't ${labels[action].button.toLowerCase()} it: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <button
      type="button"
      onClick={() => void run()}
      disabled={busy}
      aria-label={`${labels[action].button} ${item.title}`}
      className={cn(
        'min-h-9 cursor-pointer whitespace-nowrap rounded-lg border bg-transparent px-3 disabled:cursor-default disabled:opacity-50',
        action === 'undo' ? 'border-input text-foreground' : 'border-state-failed/40 text-state-failed',
        className,
      )}
    >
      {labels[action].button}
    </button>
  );
};
