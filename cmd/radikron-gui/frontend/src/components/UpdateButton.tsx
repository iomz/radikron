import React, { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';

interface UpdateAvailableData {
  version: string;
}

const checkForUpdates = () => {
  App.CheckForUpdates().catch((err) => console.error('Failed to check for updates:', err));
};

// Announces a release found by the background check. The fixed id keeps a
// repeated announcement from stacking toasts.
const announceUpdate = (version: string) => {
  toast(`Radikron ${version} is available`, {
    id: 'radikron-update',
    duration: Infinity,
    action: { label: 'Update…', onClick: checkForUpdates },
  });
};

// UpdateButton opens the update window, naming the new version once a
// background check found one. It renders nothing in builds that cannot update
// themselves (Linux and development builds).
export const UpdateButton: React.FC = () => {
  const [supported, setSupported] = useState(false);
  const [available, setAvailable] = useState('');

  useEffect(() => {
    let cancelled = false;
    const unsubscribe = Events.On('update-available', ({ data }: { data: UpdateAvailableData }) => {
      setAvailable(data.version);
      announceUpdate(data.version);
    });

    const init = async () => {
      try {
        if (!(await App.UpdatesSupported()) || cancelled) {
          return;
        }
        setSupported(true);
        // The first background check may finish before this component mounts.
        const version = await App.AvailableUpdate();
        if (version && !cancelled) {
          setAvailable(version);
          announceUpdate(version);
        }
      } catch (err) {
        console.error('Failed to read update status:', err);
      }
    };
    void init();

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  if (!supported) {
    return null;
  }
  return (
    <button
      type="button"
      onClick={checkForUpdates}
      className={cn(
        'min-h-11 rounded-[10px] px-3 cursor-pointer transition-colors',
        available
          ? 'border border-signal text-signal hover:bg-signal/10'
          : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground',
      )}
    >
      {available ? `Update to ${available}` : 'Check for updates'}
    </button>
  );
};
