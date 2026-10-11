import React from 'react';
import { Configuration } from '@/components/Configuration';
import { Stations } from '@/components/Stations';
import { Activity } from '@/components/Activity';
import { StatusPanel } from '@/components/StatusPanel';

export const Dashboard: React.FC = () => {
  return (
    <div className="flex flex-col items-center px-4 py-8 gap-6">
      <StatusPanel />
      <div className="grid gap-6 md:grid-cols-2 w-full max-w-7xl mx-auto">
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
};
