import React, { useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import {
  countByFilter,
  downloadFilters,
  downloadKey,
  downloadNote,
  downloadSource,
  downloadsList,
  filterDownloads,
  formatAirTime,
  DownloadStates,
  type DownloadFilter,
} from '@/lib/downloads';
import { StateChip } from '@/components/StateChip';
import { ProgramDetailsDialog } from '@/components/ProgramDetailsDialog';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { DownloadItem } from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const headerCell = 'px-4 py-3 font-medium';
const buttonClass = 'min-h-11 cursor-pointer rounded-[10px] px-[18px]';

// Downloads lists every scheduled program with its state; a row opens the
// program's details, and programs added from Search can be removed.
export const Downloads: React.FC = () => {
  const downloads = useAppStore((state) => state.downloads);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [filter, setFilter] = useState<DownloadFilter>('all');
  const [selected, setSelected] = useState<DownloadItem | null>(null);
  const [toRemove, setToRemove] = useState<DownloadItem | null>(null);
  const [removing, setRemoving] = useState(false);

  const items = downloadsList(downloads);
  const counts = countByFilter(items);
  const shown = filterDownloads(items, filter);

  const confirmRemove = async () => {
    if (!toRemove) return;
    setRemoving(true);
    try {
      await App.DeleteManualInjection(toRemove.id);
      addActivityLog('success', `Removed ${toRemove.title} from downloads`);
      setToRemove(null);
      await loadDownloads();
    } catch (err) {
      toast.error(`Couldn't remove it: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setRemoving(false);
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="m-0 text-[28px] font-semibold">Downloads</h1>
        <div role="group" aria-label="Filter by state" className="flex flex-wrap gap-1.5">
          {downloadFilters.map((f) => (
            <button
              key={f.id}
              type="button"
              aria-pressed={filter === f.id}
              onClick={() => setFilter(f.id)}
              className={cn(
                'min-h-9 cursor-pointer rounded-full px-3.5 text-[13px]',
                filter === f.id
                  ? 'border border-signal bg-signal/10 text-signal'
                  : 'border border-input bg-transparent text-muted-foreground hover:text-foreground',
              )}
            >
              {f.label} {counts[f.id]}
            </button>
          ))}
        </div>
      </header>

      {shown.length === 0 ? (
        <p className="m-0 rounded-[14px] border border-border px-5 py-8 text-center text-muted-foreground">
          {filter === DownloadStates.completed
            ? 'Nothing completed since Radikron started. Earlier downloads are in your download folder.'
            : items.length === 0
              ? 'Nothing scheduled. Programs your rules match show up here.'
              : 'No downloads in this state.'}
        </p>
      ) : (
        <div className="overflow-x-auto rounded-[14px] border border-border">
          <table className="w-full min-w-[760px] border-collapse">
            <thead>
              <tr className="text-left text-xs tracking-[0.5px] text-muted-foreground">
                <th scope="col" className={headerCell}>WHEN</th>
                <th scope="col" className={headerCell}>PROGRAM</th>
                <th scope="col" className={headerCell}>SOURCE</th>
                <th scope="col" className={headerCell}>STATE</th>
                <th scope="col" className={headerCell}>
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {shown.map((item) => (
                <tr key={downloadKey(item.stationId, item.start)} className="border-t border-border">
                  <td className="whitespace-nowrap px-4 py-3.5 font-mono">{formatAirTime(item.start)}</td>
                  <td className="px-4 py-3.5">
                    <button
                      type="button"
                      onClick={() => setSelected(item)}
                      className="cursor-pointer border-none bg-transparent p-0 text-left font-medium text-foreground hover:text-signal"
                    >
                      {item.title}
                    </button>
                    <div className="text-[13px] text-muted-foreground">{stationLabel(item.stationId)}</div>
                  </td>
                  <td className="px-4 py-3.5 text-muted-foreground">{downloadSource(item)}</td>
                  <td className="px-4 py-3.5">
                    <div className="flex flex-col gap-1">
                      <StateChip state={item.state} />
                      <span
                        className={cn(
                          'text-xs',
                          item.state === DownloadStates.failed ? 'text-state-failed' : 'text-muted-foreground',
                        )}
                      >
                        {downloadNote(item)}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3.5 text-right">
                    {item.manual && (
                      <button
                        type="button"
                        onClick={() => setToRemove(item)}
                        className="min-h-9 cursor-pointer rounded-lg border border-state-failed/40 bg-transparent px-3 text-state-failed"
                      >
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ProgramDetailsDialog item={selected} onClose={() => setSelected(null)} />

      <Dialog open={toRemove !== null} onOpenChange={(open) => !open && setToRemove(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Remove from downloads?</DialogTitle>
            <DialogDescription>
              {toRemove && `${toRemove.title} (${stationLabel(toRemove.stationId)}) will not be downloaded. You can add it again from Search.`}
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setToRemove(null)}
              disabled={removing}
              className={cn(buttonClass, 'border border-input bg-transparent')}
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => void confirmRemove()}
              disabled={removing}
              className={cn(buttonClass, 'border-none bg-destructive text-destructive-foreground')}
            >
              {removing ? 'Removing…' : 'Remove'}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};
