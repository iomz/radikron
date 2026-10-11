import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useAppStore } from '@/store/useAppStore';

export const Stations: React.FC = () => {
  const stations = useAppStore((state) => state.stations) ?? [];
  const stationLabel = useAppStore((state) => state.stationLabel);
  const refreshStations = useAppStore((state) => state.refreshStations);
  const sortedStations = [...stations].sort((a, b) =>
    stationLabel(a).localeCompare(stationLabel(b), undefined, { sensitivity: 'base' }) || a.localeCompare(b),
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>Stations</CardTitle>
        <CardDescription>
          Stations Radikron searches for programs: the stations in your area plus any extra stations in the configuration.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 flex flex-col h-full">
        <p className="text-sm">
          {stations.length === 0
            ? 'No stations available'
            : sortedStations.map((station) => stationLabel(station)).join(', ')}
        </p>
        <div className="mt-auto space-y-2">
          <Button variant="outline" onClick={refreshStations} className="w-full">
            Refresh station list
          </Button>
          <p className="text-xs text-muted-foreground">
            Fetches the current station list from radiko. Radikron also does this when it starts.
          </p>
        </div>
      </CardContent>
    </Card>
  );
};
