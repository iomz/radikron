import React from 'react';
import { Configuration } from '@/components/Configuration';
import { Stations } from '@/components/Stations';
import { Activity } from '@/components/Activity';

// Settings gathers the configuration, the station list, and the activity log.
// The screen layout is provisional until D2 of the GUI redesign (#77).
export const Settings: React.FC = () => (
  <div className="flex flex-col gap-6 max-w-7xl">
    <h1 className="m-0 text-[28px] font-semibold">Settings</h1>
    <div className="grid gap-6 md:grid-cols-2">
      <Configuration />
      <Stations />
      <details className="md:col-span-2">
        <summary className="cursor-pointer text-sm text-muted-foreground">Activity log</summary>
        <div className="mt-2">
          <Activity />
        </div>
      </details>
    </div>
  </div>
);
