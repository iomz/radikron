import React, { useState } from 'react';
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
import { DownloadActionButton } from '@/components/DownloadActionButton';
import { cn } from '@/lib/utils';
import type { DownloadItem } from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const headerCell = 'px-4 py-3 font-medium';

const emptyMessages: Partial<Record<DownloadFilter, string>> = {
  [DownloadStates.completed]: 'No completed downloads yet. Earlier downloads are in your download folder.',
  [DownloadStates.skipped]: 'Nothing skipped. Skip or cancel a download to keep rules from downloading it.',
};

// Downloads lists every scheduled program with its state; a row opens the
// program's details and can cancel, skip, or undo a skip.
export const Downloads: React.FC = () => {
  const downloads = useAppStore((state) => state.downloads);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const [filter, setFilter] = useState<DownloadFilter>('all');
  const [selected, setSelected] = useState<DownloadItem | null>(null);

  const items = downloadsList(downloads);
  const counts = countByFilter(items);
  const shown = filterDownloads(items, filter);

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
          {emptyMessages[filter] ??
            (items.length === 0 ? 'Nothing scheduled. Programs your rules match show up here.' : 'No downloads in this state.')}
        </p>
      ) : (
        <div className="overflow-x-auto rounded-[14px] border border-border">
          <table className="w-full min-w-[760px] border-collapse">
            <thead>
              <tr className="text-left text-xs tracking-[0.5px] text-muted-foreground">
                <th scope="col" className={headerCell}>
                  WHEN
                </th>
                <th scope="col" className={headerCell}>
                  PROGRAM
                </th>
                <th scope="col" className={headerCell}>
                  SOURCE
                </th>
                <th scope="col" className={headerCell}>
                  STATE
                </th>
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
                      <span className={cn('text-xs', item.state === DownloadStates.failed ? 'text-state-failed' : 'text-muted-foreground')}>
                        {downloadNote(item)}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3.5 text-right">
                    <DownloadActionButton item={item} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ProgramDetailsDialog item={selected} onClose={() => setSelected(null)} />
    </div>
  );
};
