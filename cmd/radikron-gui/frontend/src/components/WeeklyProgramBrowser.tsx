import React, { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useAppStore } from '@/store/useAppStore';
import * as App from '../../wailsjs/go/main/App';
import { radikron } from '../../wailsjs/go/models';

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
  const stationsRaw = useAppStore((state) => state.stations);
  // Sort stations alphabetically
  const stations = [...stationsRaw].sort((a, b) => a.localeCompare(b));
  const [searchCriteria, setSearchCriteria] = useState({
    keyword: '',
    station: '',
  });
  const [programs, setPrograms] = useState<Program[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isInitializing, setIsInitializing] = useState(true);

  // Fetch program snapshots when component mounts
  useEffect(() => {
    const fetchSnapshots = async () => {
      try {
        setIsInitializing(true);
        // @ts-ignore - FetchProgramSnapshots will be available after Wails rebuild
        await App.FetchProgramSnapshots();
      } catch (err) {
        console.error('Failed to fetch program snapshots:', err);
        const errorMessage = err instanceof Error ? err.message : String(err);
        setError(`Failed to load program data: ${errorMessage}`);
      } finally {
        setIsInitializing(false);
      }
    };

    fetchSnapshots();
  }, []);

  const performSearch = useCallback(async () => {
    // Don't search while initializing
    if (isInitializing) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      // Convert "all" back to empty string for backend
      const stationValue = searchCriteria.station === 'all' ? '' : searchCriteria.station;
      
      // @ts-ignore - SearchWeeklyPrograms will be available after Wails rebuild
      const results: any[] = await App.SearchWeeklyPrograms(
        '', // title (not used)
        '', // pfm (not used)
        searchCriteria.keyword,
        stationValue
      );

      // Handle null or undefined results
      if (!results || !Array.isArray(results)) {
        setPrograms([]);
        return;
      }

      // Convert results to Program interface
      const convertedPrograms: Program[] = results.map((prog: any) => ({
        ID: prog.ID || '',
        StationID: prog.StationID || '',
        Ft: prog.Ft || '',
        To: prog.To || '',
        Title: prog.Title || '',
        Desc: prog.Desc || '',
        Info: prog.Info || '',
        Pfm: prog.Pfm || '',
        Tags: prog.Tags || [],
        Genre: {
          Personality: prog.Genre?.Personality || '',
          Program: prog.Genre?.Program || '',
        },
      }));

      // Sort programs by start date (Ft field) - format is YYYYMMDDHHmmss, so string comparison works
      const sortedPrograms = convertedPrograms.sort((a, b) => {
        if (!a.Ft && !b.Ft) return 0;
        if (!a.Ft) return 1;
        if (!b.Ft) return -1;
        return a.Ft.localeCompare(b.Ft);
      });

      setPrograms(sortedPrograms);
    } catch (err) {
      console.error('Failed to search programs:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(`Failed to search programs: ${errorMessage}`);
    } finally {
      setIsLoading(false);
    }
  }, [searchCriteria.keyword, searchCriteria.station, isInitializing]);

  // Auto-search when criteria changes (debounced)
  useEffect(() => {
    if (isInitializing) {
      return;
    }

    const timeoutId = setTimeout(() => {
      performSearch();
    }, 300); // 300ms debounce

    return () => clearTimeout(timeoutId);
  }, [performSearch, isInitializing]);

  const handleSearch = () => {
    performSearch();
  };

  const handleClear = () => {
    setSearchCriteria({
      keyword: '',
      station: '',
    });
    setPrograms([]);
    setError(null);
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
      <Card className="w-full max-w-6xl flex flex-col h-[600px]">
        <CardHeader className="flex-shrink-0">
          <CardTitle>Program Search</CardTitle>
          <CardDescription>
            Search weekly programs using rule matching criteria. Leave fields empty to match all.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex-1 min-h-0 flex flex-col gap-4">
          <div className="flex-shrink-0 flex gap-2 items-end">
            <div className="flex-1 space-y-2">
              <Label htmlFor="keyword">Keyword</Label>
              <Input
                id="keyword"
                placeholder="e.g., シティポップ"
                value={searchCriteria.keyword}
                onChange={(e) =>
                  setSearchCriteria((prev) => ({ ...prev, keyword: e.target.value }))
                }
              />
            </div>
            <div className="w-48 space-y-2">
              <Label htmlFor="station">Station</Label>
              <Select
                value={searchCriteria.station || 'all'}
                onValueChange={(value: string) =>
                  setSearchCriteria((prev) => ({ ...prev, station: value === 'all' ? '' : value }))
                }
              >
                <SelectTrigger id="station">
                  <SelectValue placeholder="All Stations" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Stations</SelectItem>
                  {stations.map((station) => (
                    <SelectItem key={station} value={station}>
                      {station}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button onClick={handleSearch} disabled={isLoading || isInitializing}>
              {isLoading ? 'Searching...' : 'Search'}
            </Button>
            <Button onClick={handleClear} variant="outline" disabled={isLoading || isInitializing}>
              Clear
            </Button>
          </div>

          {error && (
            <div className="flex-shrink-0 p-4 rounded-md bg-destructive/10 text-destructive border border-destructive/20">
              {error}
            </div>
          )}

          {programs.length > 0 && (
            <div className="flex-1 flex flex-col min-h-0">
              <div className="flex-shrink-0 text-sm text-muted-foreground mb-2">
                Found {programs.length} matching program{programs.length !== 1 ? 's' : ''}
              </div>
              <div className="flex-1 min-h-0 overflow-hidden">
                <ScrollArea className="h-full">
                  <div className="space-y-3 pr-4">
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
                </ScrollArea>
              </div>
            </div>
          )}

          {isInitializing && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>Loading program data...</p>
            </div>
          )}

          {!error && !isInitializing && programs.length === 0 && !isLoading && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>No programs found. Try adjusting your search criteria.</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};
