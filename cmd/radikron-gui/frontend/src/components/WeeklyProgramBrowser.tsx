import React, { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useAppStore } from '@/store/useAppStore';

interface Program {
  ID: string;
  StationID: string;
  Ft: string;
  To: string;
  Title: string;
  Desc: string;
  Info: string;
  Pfm: string;
  Tags: string[];
  Genre: {
    Personality: string;
    Program: string;
  };
}

export const WeeklyProgramBrowser: React.FC = () => {
  const stations = useAppStore((state) => state.stations);
  const [selectedStation, setSelectedStation] = useState<string>('');
  const [programs, setPrograms] = useState<Program[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleFetchPrograms = async () => {
    if (!selectedStation) {
      setError('Please select a station');
      return;
    }

    setIsLoading(true);
    setError(null);
    try {
      // TODO: Add GetWeeklyPrograms method to backend
      // For now, show a placeholder message
      setError('Weekly program fetching is not yet implemented. This feature will be available soon.');
      // const progs = await App.GetWeeklyPrograms(selectedStation);
      // setPrograms(progs);
    } catch (err) {
      console.error('Failed to fetch programs:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(`Failed to fetch programs: ${errorMessage}`);
    } finally {
      setIsLoading(false);
    }
  };

  const formatDateTime = (dateTimeStr: string) => {
    if (!dateTimeStr || dateTimeStr.length !== 14) return dateTimeStr;
    // Format: 20230605130000 -> 2023-06-05 13:00
    const year = dateTimeStr.substring(0, 4);
    const month = dateTimeStr.substring(4, 6);
    const day = dateTimeStr.substring(6, 8);
    const hour = dateTimeStr.substring(8, 10);
    const minute = dateTimeStr.substring(10, 12);
    return `${year}-${month}-${day} ${hour}:${minute}`;
  };

  return (
    <div className="flex items-center justify-center px-4 py-8">
      <Card className="w-full max-w-6xl">
        <CardHeader>
          <CardTitle>Weekly Program Browser</CardTitle>
          <CardDescription>Browse weekly program schedules by station</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-4 items-end">
            <div className="flex-1 space-y-2">
              <label htmlFor="station-select" className="text-sm font-medium">
                Select Station
              </label>
              <div className="flex gap-2">
                <select
                  id="station-select"
                  value={selectedStation}
                  onChange={(e) => setSelectedStation(e.target.value)}
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                >
                  <option value="">-- Select a station --</option>
                  {stations.map((station) => (
                    <option key={station} value={station}>
                      {station}
                    </option>
                  ))}
                </select>
                <Button onClick={handleFetchPrograms} disabled={isLoading || !selectedStation}>
                  {isLoading ? 'Loading...' : 'Fetch Programs'}
                </Button>
              </div>
            </div>
          </div>

          {error && (
            <div className="p-4 rounded-md bg-destructive/10 text-destructive border border-destructive/20">
              {error}
            </div>
          )}

          {programs.length > 0 && (
            <div className="space-y-4">
              <div className="text-sm text-muted-foreground">
                Found {programs.length} program{programs.length !== 1 ? 's' : ''}
              </div>
              <div className="space-y-3 max-h-[600px] overflow-y-auto">
                {programs.map((program) => (
                  <Card key={program.ID} className="p-4">
                    <div className="space-y-2">
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <h3 className="font-semibold text-lg">{program.Title}</h3>
                          {program.Pfm && (
                            <p className="text-sm text-muted-foreground">Host: {program.Pfm}</p>
                          )}
                        </div>
                        <Badge variant="outline">{program.StationID}</Badge>
                      </div>
                      <div className="flex gap-4 text-sm text-muted-foreground">
                        <span>Start: {formatDateTime(program.Ft)}</span>
                        <span>End: {formatDateTime(program.To)}</span>
                      </div>
                      {program.Desc && (
                        <p className="text-sm text-muted-foreground line-clamp-2">{program.Desc}</p>
                      )}
                      {program.Genre && (program.Genre.Personality || program.Genre.Program) && (
                        <div className="flex gap-2 flex-wrap">
                          {program.Genre.Personality && (
                            <Badge variant="secondary">{program.Genre.Personality}</Badge>
                          )}
                          {program.Genre.Program && (
                            <Badge variant="secondary">{program.Genre.Program}</Badge>
                          )}
                        </div>
                      )}
                      {program.Tags && program.Tags.length > 0 && (
                        <div className="flex gap-2 flex-wrap">
                          {program.Tags.map((tag, idx) => (
                            <Badge key={idx} variant="outline" className="text-xs">
                              {tag}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </div>
                  </Card>
                ))}
              </div>
            </div>
          )}

          {!error && programs.length === 0 && selectedStation && !isLoading && (
            <div className="text-center py-8 text-muted-foreground">
              <p>No programs found for this station.</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

