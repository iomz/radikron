import React from 'react';
import { useAppStore } from '@/store/useAppStore';
import { nextDownloads } from '@/lib/downloads';
import { formatCheckTime } from '@/lib/status';
import { screens, type ScreenId } from '@/lib/navigation';
import { ThemeToggle } from '@/components/ThemeToggle';
import { UpdateButton } from '@/components/UpdateButton';
import { cn } from '@/lib/utils';

interface AppShellProps {
  screen: ScreenId;
  onNavigate: (screen: ScreenId) => void;
  children: React.ReactNode;
}

// TunerMark is the app mark: a dial with its needle.
const TunerMark: React.FC = () => (
  <svg width="26" height="26" viewBox="0 0 26 26" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8">
    <circle cx="13" cy="13" r="10" />
    <path d="M13 13 L19 7" />
    <circle cx="13" cy="13" r="2" fill="currentColor" />
  </svg>
);

// SidebarStatus summarizes automatic downloads at the bottom of the sidebar.
const SidebarStatus: React.FC = () => {
  const status = useAppStore((state) => state.status);
  if (!status) {
    return null;
  }
  let label = 'Downloads off';
  if (status.starting) label = 'Starting…';
  else if (status.monitoring) label = 'Downloads on';
  const next = formatCheckTime(status.nextCheck);
  const detail = status.checking ? 'Checking now…' : next ? `Next check ${next}` : '';
  const failing = Boolean(status.configError || status.stationsError);

  return (
    <div className="rounded-[10px] border border-input p-3 flex flex-col gap-1.5" aria-live="polite">
      <div className="flex items-center gap-2">
        <span
          aria-hidden="true"
          className={cn(
            'size-2 rounded-full',
            status.monitoring ? 'bg-signal shadow-[0_0_0_4px_color-mix(in_srgb,var(--color-signal)_18%,transparent)]' : 'bg-muted-foreground',
          )}
        />
        <span className="font-medium">{label}</span>
      </div>
      {detail && <span className="text-xs text-muted-foreground">{detail}</span>}
      {failing && <span className="text-xs text-state-failed">Needs attention</span>}
    </div>
  );
};

// AppShell is the window frame: the sidebar with navigation, status, updates,
// and the theme switch, next to the current screen.
export const AppShell: React.FC<AppShellProps> = ({ screen, onNavigate, children }) => {
  const downloads = useAppStore((state) => state.downloads);
  const pending = nextDownloads(downloads, Number.POSITIVE_INFINITY).length;
  const counts: Partial<Record<ScreenId, number>> = { downloads: pending };

  return (
    <div className="min-h-screen flex flex-wrap bg-background text-foreground text-sm">
      <nav
        aria-label="Main"
        className="flex-[1_1_220px] max-w-60 box-border px-3.5 py-5 bg-sidebar border-r border-border flex flex-col gap-1"
      >
        <div className="flex items-center gap-2.5 px-2.5 pt-1 pb-5 text-signal">
          <TunerMark />
          <span className="font-semibold text-[17px] text-foreground">Radikron</span>
        </div>
        {screens.map((item) => {
          const active = item.id === screen;
          const count = counts[item.id];
          return (
            <button
              key={item.id}
              type="button"
              aria-current={active ? 'page' : undefined}
              onClick={() => onNavigate(item.id)}
              className={cn(
                'flex items-center justify-between min-h-11 px-3 rounded-[10px] text-left cursor-pointer transition-colors',
                active ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground',
              )}
            >
              <span>{item.label}</span>
              {count ? <span className="font-mono text-xs text-muted-foreground">{count}</span> : null}
            </button>
          );
        })}
        <div className="grow min-h-6" />
        <div className="flex flex-col gap-2">
          <SidebarStatus />
          <UpdateButton />
          <ThemeToggle />
        </div>
      </nav>
      <main className="flex-[999_1_560px] min-w-0 box-border px-8 pt-7 pb-10">{children}</main>
    </div>
  );
};
