import React from 'react';
import { Configuration } from '@/components/Configuration';
import { Stations } from '@/components/Stations';
import { Activity } from '@/components/Activity';
import { useAppStore } from '@/store/useAppStore';

export const Dashboard: React.FC = () => {
  const configInfo = useAppStore((state) => state.configInfo);

  return (
    <div className="flex flex-col items-center px-4 py-8 gap-6">
      <p className="w-full max-w-7xl text-sm text-muted-foreground">
        {configInfo
          ? 'Radikron checks your rules and downloads matching programs while the app is open. Close the app to pause automatic downloads.'
          : 'Load a valid configuration to enable automatic downloads.'}
      </p>
      <div className="grid gap-6 md:grid-cols-2 w-full max-w-7xl mx-auto">
        <Configuration />
        <Stations />
        <Activity />
      </div>
    </div>
  );
};
