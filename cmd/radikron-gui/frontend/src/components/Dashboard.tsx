import React from 'react';
import { StatusPanel } from '@/components/StatusPanel';
import { DownloadSummary } from '@/components/DownloadSummary';

export const Dashboard: React.FC = () => {
  return (
    <div className="flex flex-col gap-6 max-w-7xl">
      <StatusPanel />
      <div className="grid gap-6 md:grid-cols-2">
        <DownloadSummary />
      </div>
    </div>
  );
};
