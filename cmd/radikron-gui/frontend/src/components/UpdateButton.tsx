import React, { useEffect, useState } from 'react';
import { CircleArrowUp } from 'lucide-react';
import { Events } from '@wailsio/runtime';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
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

// UpdateButton opens the update window. It renders nothing in builds that
// cannot update themselves (Linux and development builds).
export const UpdateButton: React.FC = () => {
  const [supported, setSupported] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const unsubscribe = Events.On('update-available', ({ data }: { data: UpdateAvailableData }) => {
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
    <Button
      variant="outline"
      size="icon"
      type="button"
      onClick={checkForUpdates}
      aria-label="Check for updates"
      title="Check for updates"
    >
      <CircleArrowUp className="h-[1.2rem] w-[1.2rem]" />
    </Button>
  );
};
