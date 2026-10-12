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
import { SanitizedHTML } from '@/components/SanitizedHTML';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Prog } from '../../../bindings/github.com/iomz/radikron/models';
import type { DownloadItem } from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const headerCell = 'px-4 py-3 font-medium';
const buttonClass = 'min-h-11 cursor-pointer rounded-[10px] px-[18px]';

function formatRange(ft: string, to: string): string {
  const t = (v: string) => (/^\d{14}$/.test(v) ? `${v.slice(8, 10)}:${v.slice(10, 12)}` : '');
  if (!/^\d{14}$/.test(ft)) return '';
  return `${ft.slice(0, 4)}/${ft.slice(4, 6)}/${ft.slice(6, 8)} ${t(ft)}–${t(to)}`;
}

// Downloads lists every scheduled program with its state; a row opens the
// program's details, and programs added from Search can be removed.
export const Downloads: React.FC = () => {
  const downloads = useAppStore((state) => state.downloads);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [filter, setFilter] = useState<DownloadFilter>('all');
  const [details, setDetails] = useState<Prog | null>(null);
  const [toRemove, setToRemove] = useState<DownloadItem | null>(null);
  const [removing, setRemoving] = useState(false);

  const items = downloadsList(downloads);
  const counts = countByFilter(items);
  const shown = filterDownloads(items, filter);

  const openDetails = async (item: DownloadItem) => {
    try {
      const schedules = (await App.GetSchedules()) ?? [];
      const prog = schedules.find((p) => p && downloadKey(p.StationID, p.Ft) === downloadKey(item.stationId, item.start));
      if (prog) setDetails(prog);
    } catch (err) {
      toast.error(`Couldn't load the program: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

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
            ? 'Nothing downloaded since Radikron started. Earlier downloads are in your download folder.'
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
                      onClick={() => void openDetails(item)}
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

      <Dialog open={details !== null} onOpenChange={(open) => !open && setDetails(null)}>
        <DialogContent className="max-h-[80vh] max-w-2xl overflow-y-auto">
          {details && (
            <>
              <DialogHeader>
                <DialogTitle>{details.Title}</DialogTitle>
                <DialogDescription>
                  {stationLabel(details.StationID)}
                  {details.Pfm && ` · ${details.Pfm}`}
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-4 text-sm">
                <span className="font-mono text-muted-foreground">{formatRange(details.Ft, details.To)}</span>
                {details.Desc && <SanitizedHTML html={details.Desc} className="leading-relaxed [&_a]:text-signal [&_a]:underline" />}
                {details.Info && <SanitizedHTML html={details.Info} className="leading-relaxed [&_a]:text-signal [&_a]:underline" />}
                {details.Tags && details.Tags.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {details.Tags.map((tag) => (
                      <span key={tag} className="rounded-full bg-muted px-2.5 py-0.5 text-xs text-muted-foreground">
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>

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
