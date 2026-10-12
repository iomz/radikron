import React, { useEffect, useState } from 'react';
import { useAppStore } from '@/store/useAppStore';
import { statusHeadline } from '@/lib/status';
import { DownloadStates, downloadNote, downloadSource, formatAirTime, nextDownloads } from '@/lib/downloads';
import { getAreaName } from '@/lib/regions';
import type { ScreenId } from '@/lib/navigation';
import { StateChip } from '@/components/StateChip';
import { DownloadActionButton } from '@/components/DownloadActionButton';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { DownloadItem } from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const LIST_LIMIT = 5;

// useNow returns the current time, refreshed every 30 seconds.
function useNow(): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 30_000);
    return () => clearInterval(id);
  }, []);
  return now;
}

function clockLine(now: Date): string {
  const day = now.toLocaleDateString('en-US', { weekday: 'short' }).toUpperCase();
  const date = `${String(now.getMonth() + 1).padStart(2, '0')}/${String(now.getDate()).padStart(2, '0')}`;
  const time = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
  return `${day} ${date} · ${time}`;
}

const sectionClass = 'flex flex-col gap-1 rounded-[14px] border border-border bg-card px-5 py-[18px]';
const linkClass = 'text-[13px] text-signal hover:underline cursor-pointer bg-transparent border-none p-0';

const NextRow: React.FC<{ item: DownloadItem; now: Date }> = ({ item, now }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const downloading = item.state === DownloadStates.downloading;
  const progress = downloading && item.total > 0 ? Math.round((item.completed / item.total) * 100) : 0;
  return (
    <div className="grid grid-cols-[72px_minmax(0,1fr)] gap-3.5 border-t border-border py-3">
      <span className="font-mono text-lg">{formatAirTime(item.start, now)}</span>
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{item.title}</span>
          <StateChip state={item.state} />
          {downloading && <DownloadActionButton item={item} className="ml-auto min-h-8 px-2.5 text-xs" />}
        </div>
        <span className="text-[13px] text-muted-foreground">
          {stationLabel(item.stationId)} · {downloadSource(item)}
        </span>
        {downloading ? (
          <div className="flex flex-col gap-1.5 pt-1">
            <div className="h-1 rounded-sm bg-input">
              <div className="h-1 rounded-sm bg-signal" style={{ width: `${progress}%` }} />
            </div>
            <span className="font-mono text-xs text-signal">{downloadNote(item)}</span>
          </div>
        ) : (
          <span className={cn('text-xs', item.state === DownloadStates.failed ? 'text-state-failed' : 'text-muted-foreground')}>
            {downloadNote(item)}
          </span>
        )}
      </div>
    </div>
  );
};

const RecentRow: React.FC<{ item: DownloadItem; now: Date }> = ({ item, now }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const ok = item.state === DownloadStates.completed;
  const finished = item.updatedAt ? new Date(item.updatedAt) : null;
  const when = finished
    ? finished.toDateString() === now.toDateString()
      ? finished.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })
      : finished.toLocaleDateString([], { month: '2-digit', day: '2-digit' })
    : '';
  return (
    <div className="grid grid-cols-[22px_minmax(0,1fr)] gap-3 border-t border-border py-3">
      <span
        aria-hidden="true"
        className={cn(
          'grid size-[22px] place-items-center rounded-full text-xs font-bold',
          ok ? 'bg-state-completed/15 text-state-completed' : 'bg-state-failed/15 text-state-failed',
        )}
      >
        {ok ? '✓' : '!'}
      </span>
      <div className="flex min-w-0 flex-col gap-1">
        <span className="font-medium">{item.title}</span>
        <span className="text-[13px] text-muted-foreground">
          {stationLabel(item.stationId)}
          {when && ` · ${when}`}
        </span>
        <span className={cn('text-xs', ok ? 'text-muted-foreground' : 'text-state-failed')}>
          <span className="sr-only">{ok ? 'Downloaded. ' : 'Failed. '}</span>
          {ok ? 'Downloaded' : downloadNote(item)}
        </span>
      </div>
    </div>
  );
};

// Overview answers what Radikron is doing, what comes next, and what just
// happened.
export const Overview: React.FC<{ onNavigate: (screen: ScreenId) => void }> = ({ onNavigate }) => {
  const status = useAppStore((state) => state.status);
  const downloads = useAppStore((state) => state.downloads);
  const configInfo = useAppStore((state) => state.configInfo);
  const stations = useAppStore((state) => state.stations);
  const retry = useAppStore((state) => state.retry);
  const refreshStations = useAppStore((state) => state.refreshStations);
  const now = useNow();
  const [retrying, setRetrying] = useState(false);

  const activity = {
    downloading: (downloads?.scheduled ?? []).filter((d) => d.state === DownloadStates.downloading).map((d) => d.title),
    pending: nextDownloads(downloads, Number.POSITIVE_INFINITY).length,
  };
  const headline = status ? statusHeadline(status, now, activity) : { title: 'Starting up…', detail: '' };
  const errors = [
    status?.configError && `Couldn't load the configuration: ${status.configError}`,
    status?.stationsError && `Couldn't load the station list: ${status.stationsError}. Downloads keep using the last list.`,
  ].filter(Boolean) as string[];
  const next = nextDownloads(downloads, LIST_LIMIT);
  const recent = (downloads?.recent ?? []).slice(0, LIST_LIMIT);
  const area = configInfo?.AreaID ? `${getAreaName(configInfo.AreaID) ?? configInfo.AreaID} (${configInfo.AreaID})` : '';
  const rules = configInfo?.Rules?.length ?? 0;

  const handleRetry = async () => {
    setRetrying(true);
    try {
      await retry();
    } finally {
      setRetrying(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1.5">
          <span className="font-mono text-xs tracking-[1px] text-muted-foreground">{clockLine(now)}</span>
          <h1 className="m-0 text-[28px] font-semibold leading-tight">{headline.title}</h1>
          {headline.detail && <span className="text-muted-foreground">{headline.detail}</span>}
        </div>
        <button
          type="button"
          onClick={() => void App.CheckNow()}
          disabled={!status?.monitoring || status.checking}
          className="min-h-11 cursor-pointer rounded-[10px] border border-input bg-secondary px-[18px] disabled:cursor-default disabled:opacity-50"
        >
          Check now
        </button>
      </header>

      {errors.length > 0 && (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-xl border border-state-failed/40 bg-state-failed/10 px-4 py-3.5 text-state-failed"
        >
          <div className="flex flex-[1_1_320px] flex-col gap-1">
            {errors.map((message) => (
              <span key={message}>{message}</span>
            ))}
          </div>
          <button
            type="button"
            onClick={handleRetry}
            disabled={retrying}
            className="min-h-10 cursor-pointer rounded-lg border border-current bg-transparent px-4"
          >
            {retrying ? 'Retrying…' : 'Retry'}
          </button>
        </div>
      )}

      <div className="grid grid-cols-[repeat(auto-fit,minmax(320px,1fr))] gap-5">
        <section aria-labelledby="next-heading" className={sectionClass}>
          <div className="flex items-baseline justify-between pb-2">
            <h2 id="next-heading" className="m-0 text-[15px] font-semibold">
              Next up
            </h2>
            <button type="button" className={linkClass} onClick={() => onNavigate('downloads')}>
              All downloads
            </button>
          </div>
          {next.length === 0 ? (
            <p className="m-0 border-t border-border py-3 text-muted-foreground">Nothing scheduled. Programs your rules match show up here.</p>
          ) : (
            next.map((item) => <NextRow key={`${item.stationId}|${item.start}`} item={item} now={now} />)
          )}
        </section>

        <section aria-labelledby="recent-heading" className={sectionClass}>
          <div className="flex items-baseline justify-between pb-2">
            <h2 id="recent-heading" className="m-0 text-[15px] font-semibold">
              Recent results
            </h2>
            <span className="text-xs text-muted-foreground">Since Radikron started</span>
          </div>
          {recent.length === 0 ? (
            <p className="m-0 border-t border-border py-3 text-muted-foreground">No downloads have finished yet.</p>
          ) : (
            recent.map((item) => <RecentRow key={`${item.stationId}|${item.start}`} item={item} now={now} />)
          )}
        </section>
      </div>

      <section
        aria-label="Setup"
        className="flex flex-wrap gap-x-8 gap-y-3 rounded-[14px] border border-dashed border-input px-5 py-4 text-[13px] text-muted-foreground"
      >
        <span>
          {area && <span className="text-foreground">{area}</span>}
          {area && ' · '}
          {stations.length} stations ·{' '}
          <button type="button" className={linkClass} onClick={() => void refreshStations()}>
            Refresh list
          </button>
        </span>
        <span>
          <span className="text-foreground">
            {rules} {rules === 1 ? 'rule' : 'rules'}
          </span>{' '}
          ·{' '}
          <button type="button" className={linkClass} onClick={() => onNavigate('rules')}>
            Edit rules
          </button>
        </span>
        {configInfo?.DownloadDir && (
          <span>
            Saves to <span className="font-mono text-foreground">{configInfo.DownloadDir}</span>
          </span>
        )}
        <button type="button" className={linkClass} onClick={() => onNavigate('settings')}>
          Activity log
        </button>
      </section>
    </div>
  );
};
