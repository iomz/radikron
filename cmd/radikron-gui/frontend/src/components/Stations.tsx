import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useAppStore } from '@/store/useAppStore';

export const Stations: React.FC = () => {
  const stations = useAppStore((state) => state.stations);
  const refreshStations = useAppStore((state) => state.refreshStations);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Stations</CardTitle>
        <CardDescription>Available radio stations</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-2">
          {stations.length === 0 ? (
            <Badge variant="outline" aria-label="No stations available">
              No stations available
            </Badge>
          ) : (
            stations.map((station) => (
              <Badge key={station} variant="outline">
                {station}
              </Badge>
            ))
          )}
        </div>
        <Button variant="outline" onClick={refreshStations} className="w-full">
          Refresh Stations
        </Button>
      </CardContent>
    </Card>
  );
};

