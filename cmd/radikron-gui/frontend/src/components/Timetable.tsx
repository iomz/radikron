import React, { useEffect, useState } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useAppStore } from '@/store/useAppStore';
import { DownloadStates, downloadNote, downloadsList, downloadStateLabel, downloadSource } from '@/lib/downloads';
import { StateChip } from '@/components/StateChip';
import { ProgramDetailsDialog } from '@/components/ProgramDetailsDialog';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import { DAY_HOURS, DAY_START_HOUR, buildLanes, formatDay, position, scheduledDays, todayBroadcastDay } from '@/lib/timetable';
import { cn } from '@/lib/utils';

const blockStyles: Record<string, string> = {
  [DownloadStates.downloading]: 'bg-signal text-primary-foreground',
  [DownloadStates.available]: 'bg-card border-2 border-state-available text-state-available',
  [DownloadStates.waiting]: 'bg-input text-foreground',
  [DownloadStates.failed]: 'bg-state-failed/15 border-2 border-state-failed text-state-failed',
  [DownloadStates.completed]: 'bg-state-completed/20 text-state-completed',
  [DownloadStates.expired]: 'bg-muted text-state-expired line-through',
  [DownloadStates.skipped]: 'border border-dashed border-input bg-transparent text-muted-foreground',
};

const legendStates = [
  DownloadStates.downloading,
  DownloadStates.available,
  DownloadStates.waiting,
  DownloadStates.failed,
  DownloadStates.completed,
  DownloadStates.skipped,
];

// Tick marks every two hours, labelled in clock time (26:00 shows as 02:00).
const ticks = Array.from({ length: DAY_HOURS / 2 - 1 }, (_, i) => DAY_START_HOUR + 1 + i * 2);

function nowValue(now: Date): string {
  const jst = new Date(now.getTime() + 9 * 3600_000);
  const p = (n: number) => String(n).padStart(2, '0');
  return `${jst.getUTCFullYear()}${p(jst.getUTCMonth() + 1)}${p(jst.getUTCDate())}${p(jst.getUTCHours())}${p(jst.getUTCMinutes())}00`;
}

function hhmm(value: string): string {
  return `${value.slice(8, 10)}:${value.slice(10, 12)}`;
}

const CARD_WIDTH = 280;

interface Hovered {
  item: DownloadItem;
  left: number;
  top: number;
}

// BlockCard is the hover card for a timetable block. It is fixed to the
// window so the timetable's scroll box does not clip it.
const BlockCard: React.FC<Hovered> = ({ item, left, top }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  return (
    <div
      role="tooltip"
      className="pointer-events-none fixed z-50 flex flex-col gap-1.5 rounded-xl border border-input bg-popover p-3 text-sm text-popover-foreground shadow-lg"
      style={{ left, top, width: CARD_WIDTH }}
    >
      <span className="font-medium">{item.title}</span>
      <span className="text-[13px] text-muted-foreground">
        {stationLabel(item.stationId)} · <span className="font-mono">{hhmm(item.start)}–{hhmm(item.end)}</span>
      </span>
      <div className="flex flex-wrap items-center gap-2">
        <StateChip state={item.state} />
        <span className="text-xs text-muted-foreground">{downloadSource(item)}</span>
      </div>
      <span className="text-xs text-muted-foreground">{downloadNote(item)}</span>
    </div>
  );
};

// Timetable lays the scheduled programs of one broadcast day (05:00–29:00,
// Japan time) on a time axis with one lane per station.
export const Timetable: React.FC = () => {
  const downloads = useAppStore((state) => state.downloads);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(id);
  }, []);

  const items = downloadsList(downloads);
  const today = todayBroadcastDay(now);
  const days = [...new Set([today, ...scheduledDays(items)])].sort();
  const [day, setDay] = useState(today);
  const index = days.indexOf(day);
  const lanes = buildLanes(items, day);
  const nowLeft = day === today ? position(nowValue(now), day) : null;
  const [hovered, setHovered] = useState<Hovered | null>(null);
  const [selected, setSelected] = useState<DownloadItem | null>(null);

  const showCard = (item: DownloadItem) => (e: React.MouseEvent<HTMLElement> | React.FocusEvent<HTMLElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const left = Math.min(Math.max(8, rect.left), window.innerWidth - CARD_WIDTH - 8);
    setHovered({ item, left, top: rect.bottom + 6 });
  };

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="m-0 text-[28px] font-semibold">Timetable</h1>
        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label="Previous day"
            disabled={index <= 0}
            onClick={() => setDay(days[index - 1])}
            className="grid size-10 cursor-pointer place-items-center rounded-[10px] border border-input bg-transparent disabled:cursor-default disabled:opacity-40"
          >
            <ChevronLeft className="size-4" aria-hidden="true" />
          </button>
          <span className="min-w-28 text-center font-mono">{formatDay(day)}</span>
          <button
            type="button"
            aria-label="Next day"
            disabled={index >= days.length - 1}
            onClick={() => setDay(days[index + 1])}
            className="grid size-10 cursor-pointer place-items-center rounded-[10px] border border-input bg-transparent disabled:cursor-default disabled:opacity-40"
          >
            <ChevronRight className="size-4" aria-hidden="true" />
          </button>
          <button
            type="button"
            disabled={day === today}
            onClick={() => setDay(today)}
            className="min-h-10 cursor-pointer rounded-[10px] border border-input bg-transparent px-3.5 disabled:cursor-default disabled:opacity-40"
          >
            Today
          </button>
        </div>
      </header>

      <section aria-label={`Programs on ${formatDay(day)}`} className="flex flex-col gap-3 rounded-[14px] border border-border bg-card px-5 py-[18px]">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 className="m-0 text-[15px] font-semibold">Broadcast day</h2>
          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
            {legendStates.map((state) => (
              <span key={state} className="inline-flex items-center gap-1.5">
                <span aria-hidden="true" className={cn('box-border h-2.5 w-3.5 rounded-[3px]', blockStyles[state])} />
                {downloadStateLabel(state)}
              </span>
            ))}
          </div>
        </div>

        {lanes.length === 0 ? (
          <p className="m-0 py-6 text-center text-muted-foreground">No programs scheduled on this day.</p>
        ) : (
          <div className="overflow-x-auto">
            <div className="grid min-w-[760px] grid-cols-[120px_minmax(0,1fr)] gap-y-2.5">
              <span />
              <div className="relative h-[18px] text-[11px] text-muted-foreground" aria-hidden="true">
                {ticks.map((h) => (
                  <span key={h} className="absolute top-0 -translate-x-1/2 font-mono" style={{ left: `${((h - DAY_START_HOUR) / DAY_HOURS) * 100}%` }}>
                    {String(h % 24).padStart(2, '0')}:00
                  </span>
                ))}
              </div>
              {lanes.map((lane) => (
                <React.Fragment key={lane.stationId}>
                  <span className="self-center truncate pr-2 font-medium">{stationLabel(lane.stationId)}</span>
                  <ul className="relative m-0 h-11 list-none rounded-lg bg-background p-0">
                    {nowLeft !== null && nowLeft >= 0 && nowLeft <= 100 && (
                      <li aria-hidden="true" className="absolute -bottom-1 -top-1 w-0.5 bg-signal" style={{ left: `${nowLeft}%` }} />
                    )}
                    {lane.blocks.map(({ item, left, width }) => (
                      <li key={item.start} className="absolute bottom-1.5 top-1.5" style={{ left: `${left}%`, width: `${width}%` }}>
                        <button
                          type="button"
                          onClick={() => setSelected(item)}
                          onMouseEnter={showCard(item)}
                          onMouseLeave={() => setHovered(null)}
                          onFocus={showCard(item)}
                          onBlur={() => setHovered(null)}
                          className={cn(
                            'box-border flex size-full cursor-pointer items-center overflow-hidden rounded-md px-2 text-left text-xs hover:brightness-110 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                            blockStyles[item.state],
                          )}
                        >
                          <span className="truncate">{item.title}</span>
                          <span className="sr-only">
                            , {hhmm(item.start)} to {hhmm(item.end)}, {downloadStateLabel(item.state)}
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </React.Fragment>
              ))}
            </div>
          </div>
        )}
        <span className="text-xs text-muted-foreground">
          Hover a program for a summary, or click it for details. The amber line is now. Times are Japan time; the broadcast day runs from 05:00 to 05:00. A recording becomes available 5 minutes after its program ends.
        </span>
      </section>
      {hovered && <BlockCard {...hovered} />}
      <ProgramDetailsDialog
        item={selected}
        onClose={() => {
          setSelected(null);
          setHovered(null);
        }}
      />
    </div>
  );
};
