import React, { useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { dangerButton, secondaryButton } from '@/components/form';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Downloads } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

// historyCounts counts the finished downloads and skipped programs Radikron
// remembers.
export const historyCounts = (downloads: Downloads | null) => ({
  finished: downloads?.recent?.length ?? 0,
  skipped: downloads?.skipped?.length ?? 0,
});

// ClearHistoryButton forgets finished and skipped downloads after the user
// confirms. It is disabled when there is nothing to forget.
export const ClearHistoryButton: React.FC<{ className: string }> = ({ className }) => {
  const downloads = useAppStore((state) => state.downloads);
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const { finished, skipped } = historyCounts(downloads);

  const clear = async () => {
    setBusy(true);
    try {
      await App.ClearHistory();
      await loadDownloads();
      addActivityLog('success', 'Cleared the download history');
      setOpen(false);
    } catch (err) {
      toast.error(`Couldn't clear the history: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button type="button" className={className} onClick={() => setOpen(true)} disabled={finished + skipped === 0}>
        Clear history
      </button>
      <Dialog open={open} onOpenChange={(isOpen) => !busy && setOpen(isOpen)}>
        <DialogContent className="max-w-[460px]">
          <DialogHeader>
            <DialogTitle>Clear the download history?</DialogTitle>
            <DialogDescription>
              Radikron forgets completed, failed, and skipped downloads. Your recordings stay in the download folder.
            </DialogDescription>
          </DialogHeader>
          <p className="m-0 text-[14px]">
            Rules may download skipped programs at the next check, which starts right away. Downloads in progress go on.
          </p>
          <div className="flex justify-end gap-2">
            <button type="button" className={secondaryButton} onClick={() => setOpen(false)} disabled={busy}>
              Cancel
            </button>
            <button type="button" className={dangerButton} onClick={() => void clear()} disabled={busy}>
              {busy ? 'Clearing…' : 'Clear history'}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
};
