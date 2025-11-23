import React from 'react';
import { Configuration } from '@/components/Configuration';
import { Stations } from '@/components/Stations';
import { Activity } from '@/components/Activity';

export const Dashboard: React.FC = () => {
  return (
    <div className="flex items-center justify-center px-4 py-8">
      <div className="grid gap-6 md:grid-cols-2 w-full max-w-7xl mx-auto">
        <Configuration />
        <Stations />
        <Activity />
      </div>
    </div>
  );
};

