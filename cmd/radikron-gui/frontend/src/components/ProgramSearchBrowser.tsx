import React, { useState, useEffect, useCallback } from 'react';
import DOMPurify from 'dompurify';
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import * as App from '../../wailsjs/go/main/App';
import { radikron } from '../../wailsjs/go/models';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

// SanitizedHTML renders HTML content safely with XSS protection
interface SanitizedHTMLProps {
  html: string;
  className?: string;
}

const SanitizedHTML: React.FC<SanitizedHTMLProps> = ({ html, className }) => {
  const sanitizedHTML = DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'u', 'a', 'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6'],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
  });

  const handleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    const link = target.closest('a');
    if (link && link.href) {
      e.preventDefault();
      BrowserOpenURL(link.href);
    }
  };

  return (
    <div
      className={className}
      dangerouslySetInnerHTML={{ __html: sanitizedHTML }}
      onClick={handleClick}
    />
  );
};

export const ProgramSearchBrowser: React.FC = () => {
  const stationsRaw = useAppStore((state) => state.stations);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  // Sort stations alphabetically
  const stations = [...stationsRaw].sort((a, b) => a.localeCompare(b));
  const [searchCriteria, setSearchCriteria] = useState({
    keyword: '',
    station: '',
  });
  const [programs, setPrograms] = useState<radikron.Prog[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isInitializing, setIsInitializing] = useState(true);
  const [selectedProgram, setSelectedProgram] = useState<radikron.Prog | null>(null);
  const [showInjectionDialog, setShowInjectionDialog] = useState(false);
  const [selectedRule, setSelectedRule] = useState<string>('');
  const [availableRules, setAvailableRules] = useState<Array<{ name: string; folder: string }>>([]);
  const [designatedFolder, setDesignatedFolder] = useState<string>('');
  const [isInjecting, setIsInjecting] = useState(false);

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

    // Don't search if keyword is empty
    if (!searchCriteria.keyword.trim()) {
      setPrograms([]);
      setError(null);
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

      // Convert results to radikron.Prog using the generated model
      const convertedPrograms: radikron.Prog[] = results.map((prog: any) =>
        radikron.Prog.createFrom(prog)
      );

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

  const updateDesignatedFolder = useCallback(async (prog: radikron.Prog | null, rules: Array<{ name: string; folder: string }>, ruleName: string) => {
    if (!prog) {
      setDesignatedFolder('');
      return;
    }

    try {
      // Get config to access DownloadDir
      const cfg = await App.GetConfig();
      const downloadDir = cfg.DownloadDir || 'radiko';
      
      // Find the selected rule's folder
      let folder = '';
      if (ruleName) {
        const rule = rules.find((r) => r.name === ruleName);
        if (rule) {
          folder = rule.folder || '';
        }
      }
      
      // Calculate the full path
      // @ts-ignore - GetDesignatedFolder will be available after Wails rebuild
      const fullPath = await App.GetDesignatedFolder(prog, folder, downloadDir);
      setDesignatedFolder(fullPath);
    } catch (err) {
      console.error('Failed to get designated folder:', err);
      // Fallback calculation with protected config access
      let downloadDir = 'radiko';
      try {
        const cfg = await App.GetConfig();
        downloadDir = cfg.DownloadDir || 'radiko';
      } catch (configErr) {
        console.error('Failed to get config in fallback:', configErr);
        // Use safe default 'radiko' already set above
      }
      
      // Compute rule/folder and fullPath before calling setDesignatedFolder
      const rule = rules.find((r) => r.name === ruleName);
      const folder = rule?.folder || '';
      const fullPath = folder ? `${downloadDir}/${folder}` : downloadDir;
      setDesignatedFolder(fullPath);
    }
  }, []);

  const loadRulesAndShowDialog = useCallback(async () => {
    try {
      const cfg = await App.GetConfig();
      const rules = cfg.Rules || [];
      const rulesList = rules.map((rule: any) => ({
        name: rule.Name || '',
        folder: rule.Folder || '',
      }));
      setAvailableRules(rulesList);
      
      // Try to find a matching rule
      let initialRule = '';
      if (selectedProgram) {
        const matchingRule = rules.find((rule: any) => {
          // Simple matching logic - check if rule matches the program
          if (rule.StationID && rule.StationID !== selectedProgram.StationID) {
            return false;
          }
          if (rule.Title && !selectedProgram.Title.includes(rule.Title)) {
            return false;
          }
          if (rule.Pfm && rule.Pfm !== selectedProgram.Pfm) {
            return false;
          }
          if (rule.Keyword && !selectedProgram.Title.includes(rule.Keyword) && 
              !selectedProgram.Desc?.includes(rule.Keyword)) {
            return false;
          }
          return true;
        });
        
        if (matchingRule) {
          initialRule = matchingRule.Name || '';
        }
      }
      
      setSelectedRule(initialRule);
      setShowInjectionDialog(true);
      
      // Update folder after dialog is shown
      if (selectedProgram) {
        updateDesignatedFolder(selectedProgram, rulesList, initialRule);
      }
    } catch (err) {
      console.error('Failed to load rules:', err);
      setError('Failed to load rules');
    }
  }, [selectedProgram, updateDesignatedFolder]);

  const handleRuleChange = useCallback((value: string) => {
    // Convert __default__ back to empty string
    const ruleName = value === '__default__' ? '' : value;
    setSelectedRule(ruleName);
    if (selectedProgram) {
      updateDesignatedFolder(selectedProgram, availableRules, ruleName);
    }
  }, [selectedProgram, availableRules, updateDesignatedFolder]);

  const handleInjectProgram = useCallback(async () => {
    if (!selectedProgram) return;

    setIsInjecting(true);
    setError(null);

    try {
      // @ts-ignore - InjectProgram will be available after Wails rebuild
      await App.InjectProgram(selectedProgram, selectedRule);
      setShowInjectionDialog(false);
      setSelectedProgram(null);
      setSelectedRule('');
      // Show success message
      const successMessage = `Program [${selectedProgram.StationID}]${selectedProgram.Title} added to schedule`;
      addActivityLog("success", successMessage);
      toast.success(successMessage);
    } catch (err) {
      console.error('Failed to inject program:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(`Failed to add program to schedule: ${errorMessage}`);
    } finally {
      setIsInjecting(false);
    }
  }, [selectedProgram, selectedRule]);

  return (
    <div className="flex items-center justify-center px-4 py-8">
      <Card className="w-full max-w-6xl flex flex-col h-[600px]">
        <CardHeader className="flex-shrink-0">
          <CardTitle>Program Search</CardTitle>
          <CardDescription>
            Enter a keyword to search programs. Station is optional.
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
                    <Card
                      key={program.ID}
                      className="p-4 cursor-pointer hover:bg-accent transition-colors"
                      role="button"
                      tabIndex={0}
                      onClick={() => setSelectedProgram(program)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          setSelectedProgram(program);
                        }
                      }}
                    >
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

          {!error && !isInitializing && programs.length === 0 && !isLoading && searchCriteria.keyword.trim() && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>No programs found. Try adjusting your search keyword or station.</p>
            </div>
          )}

          {!error && !isInitializing && programs.length === 0 && !isLoading && !searchCriteria.keyword.trim() && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>Enter a keyword above to search for programs.</p>
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={selectedProgram !== null} onOpenChange={(open) => !open && setSelectedProgram(null)}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto [&>button]:text-foreground [&>button:hover]:text-foreground [&>button]:opacity-100 [&>button>svg]:text-foreground">
          {selectedProgram && (
            <>
              <DialogHeader>
                <DialogTitle>{selectedProgram.Title}</DialogTitle>
                <DialogDescription>
                  <div className="flex items-center gap-2 mt-2">
                    <Badge variant="outline">{selectedProgram.StationID}</Badge>
                    {selectedProgram.Pfm && (
                      <span className="text-sm text-foreground">Host: {selectedProgram.Pfm}</span>
                    )}
                  </div>
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">Start Time</p>
                    <p className="text-sm text-foreground">{formatDateTime(selectedProgram.Ft)}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-muted-foreground">End Time</p>
                    <p className="text-sm text-foreground">{formatDateTime(selectedProgram.To)}</p>
                  </div>
                </div>

                {selectedProgram.Desc && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Description</p>
                    <SanitizedHTML
                      html={selectedProgram.Desc}
                      className="text-sm text-foreground [&_*]:text-foreground [&_a]:text-primary [&_a]:underline [&_a:hover]:opacity-80"
                    />
                  </div>
                )}

                {selectedProgram.Info && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Info</p>
                    <SanitizedHTML
                      html={selectedProgram.Info}
                      className="text-sm text-foreground [&_*]:text-foreground [&_a]:text-primary [&_a]:underline [&_a:hover]:opacity-80"
                    />
                  </div>
                )}

                {selectedProgram.Genre && (selectedProgram.Genre.Personality || selectedProgram.Genre.Program) && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-2">Genre</p>
                    <div className="flex gap-2 flex-wrap">
                      {selectedProgram.Genre.Personality && (
                        <Badge variant="secondary">{selectedProgram.Genre.Personality}</Badge>
                      )}
                      {selectedProgram.Genre.Program && (
                        <Badge variant="secondary">{selectedProgram.Genre.Program}</Badge>
                      )}
                    </div>
                  </div>
                )}

                {selectedProgram.Tags && selectedProgram.Tags.length > 0 && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-2">Tags</p>
                    <div className="flex gap-2 flex-wrap">
                      {selectedProgram.Tags.map((tag, idx) => (
                        <Badge key={idx} variant="outline" className="text-xs">
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}

                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-1">Program ID</p>
                  <p className="text-sm font-mono text-xs text-foreground">{selectedProgram.ID}</p>
                </div>

                <div className="flex justify-end pt-4 border-t">
                  <Button
                    onClick={() => {
                      loadRulesAndShowDialog();
                    }}
                    disabled={isInjecting}
                  >
                    Add to Scheduled Downloads
                  </Button>
                </div>
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Injection Dialog */}
      <Dialog open={showInjectionDialog} onOpenChange={setShowInjectionDialog}>
        <DialogContent className="max-w-2xl [&>button]:text-foreground [&>button:hover]:text-foreground [&>button]:opacity-100 [&>button>svg]:text-foreground">
          <DialogHeader>
            <DialogTitle>Add to Scheduled Downloads</DialogTitle>
            <DialogDescription>
              Select a rule to determine the download folder, or use the default folder.
            </DialogDescription>
          </DialogHeader>
          {selectedProgram && (
            <div className="space-y-4">
              <div>
                <p className="text-sm font-medium text-muted-foreground mb-1">Program</p>
                <p className="text-sm text-foreground font-semibold">{selectedProgram.Title}</p>
                <p className="text-xs text-muted-foreground">
                  {selectedProgram.StationID} • {formatDateTime(selectedProgram.Ft)}
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="rule-select">Rule (for folder organization)</Label>
                <Select value={selectedRule || '__default__'} onValueChange={handleRuleChange}>
                  <SelectTrigger id="rule-select" className="text-foreground">
                    <SelectValue placeholder="Select a rule or use default" className="text-foreground placeholder:text-muted-foreground" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__default__">Default (no rule)</SelectItem>
                    {availableRules.map((rule) => (
                      <SelectItem key={rule.name} value={rule.name}>
                        {rule.name} {rule.folder ? `(${rule.folder})` : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div>
                <p className="text-sm font-medium text-muted-foreground mb-1">Designated Folder</p>
                <p className="text-sm font-mono text-xs text-foreground bg-muted p-2 rounded">
                  {designatedFolder || 'Calculating...'}
                </p>
              </div>

              <div className="flex justify-end gap-2 pt-4 border-t">
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowInjectionDialog(false);
                    setSelectedRule('');
                  }}
                  disabled={isInjecting}
                  className="border-foreground/20 text-foreground hover:bg-accent hover:text-accent-foreground"
                >
                  Cancel
                </Button>
                <Button onClick={handleInjectProgram} disabled={isInjecting}>
                  {isInjecting ? 'Adding...' : 'Add to Schedule'}
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};
