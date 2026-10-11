import React, { useState } from 'react';
import { Button } from '@/components/ui/button';
import { useAppStore } from '@/store/useAppStore';
import { formatCheckTime } from '@/lib/status';

// StatusPanel shows whether automatic downloads run, when Radikron checks for
// programs, and failures that need attention. The layout is provisional until
// the GUI redesign (#77).
export const StatusPanel: React.FC = () => {
  const status = useAppStore((state) => state.status);
  const retry = useAppStore((state) => state.retry);
  const [retrying, setRetrying] = useState(false);

  if (!status) {
    return null;
  }

  const handleRetry = async () => {
    setRetrying(true);
    try {
      await retry();
    } finally {
      setRetrying(false);
    }
  };

  let state = 'Automatic downloads are off';
  if (status.starting) {
    state = 'Loading configuration…';
  } else if (status.monitoring) {
    state = 'Automatic downloads are on';
  }

  const times: string[] = [];
  if (status.checking) {
    times.push('Checking now…');
  } else {
    const last = formatCheckTime(status.lastCheck);
    const next = formatCheckTime(status.nextCheck);
    if (last) times.push(`Last check ${last}`);
    if (next) times.push(`Next check ${next}`);
  }

  const errors = [
    status.configError && `Couldn't load the configuration: ${status.configError}`,
    status.stationsError && `Couldn't load the station list: ${status.stationsError}`,
  ].filter(Boolean) as string[];

  return (
    <section aria-label="Status" className="w-full max-w-7xl space-y-2">
      <div className="text-sm">
        <span className="font-medium">{state}</span>
        {times.length > 0 && <span className="text-muted-foreground"> · {times.join(' · ')}</span>}
      </div>
      <p className="text-sm text-muted-foreground">
        Radikron checks your rules only while the app is open.
        {status.monitoring && !status.hasRules && ' No rule has search criteria yet, so nothing will be downloaded.'}
      </p>
      {errors.length > 0 && (
        <div role="alert" className="flex items-start gap-3 rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          <div className="flex-1 space-y-1">
            {errors.map((message) => (
              <p key={message}>{message}</p>
            ))}
          </div>
          <Button size="sm" variant="outline" onClick={handleRetry} disabled={retrying}>
            {retrying ? 'Retrying…' : 'Retry'}
          </Button>
        </div>
      )}
    </section>
  );
};
