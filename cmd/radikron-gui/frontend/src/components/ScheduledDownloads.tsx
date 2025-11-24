import React, { useState, useEffect, useCallback, useRef } from 'react';
import DOMPurify from 'dompurify';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { EventsOn } from '../../wailsjs/runtime/runtime';
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

export const ScheduledDownloads: React.FC = () => {
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [schedules, setSchedules] = useState<radikron.Prog[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedProgram, setSelectedProgram] = useState<radikron.Prog | null>(null);
  const [programToDelete, setProgramToDelete] = useState<radikron.Prog | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const isFetchingRef = useRef(false);

  const loadSchedules = useCallback(async () => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;
    setIsLoading(true);
    setError(null);
    
    let sortedSchedules: radikron.Prog[] = [];
    
    try {
      const results: any[] = await App.GetSchedules();
      const convertedSchedules: radikron.Prog[] = results.map((prog: any) =>
        radikron.Prog.createFrom(prog)
      );
      
      // Sort programs by start date (Ft field) - format is YYYYMMDDHHmmss, so string comparison works
      sortedSchedules = convertedSchedules.sort((a, b) => {
        if (!a.Ft && !b.Ft) return 0;
        if (!a.Ft) return 1;
        if (!b.Ft) return -1;
        return a.Ft.localeCompare(b.Ft);
      });
      
      setSchedules(sortedSchedules);
    } catch (err) {
      console.error('Failed to load schedules:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(`Failed to load scheduled downloads: ${errorMessage}`);
      isFetchingRef.current = false;
      setIsLoading(false);
      throw err; // Re-throw to allow callers to handle failures
    }
    
    // Check which programs are manually injected (separate try-catch to avoid blocking schedule updates)
    try {
      for (const prog of sortedSchedules) {
        try {
          // @ts-ignore - IsManualInjection will be available after Wails rebuild
          const isManual = await App.IsManualInjection(prog.ID);
          if (isManual) {
            prog.IsManualInjection = true;
          }
        } catch (err) {
          console.error(`Failed to check if program ${prog.ID} is manual injection:`, err);
        }
      }
      // Update schedules with manual injection flags
      setSchedules([...sortedSchedules]);
    } catch (err) {
      console.error('Failed to check manual injections:', err);
    }
    
    isFetchingRef.current = false;
    setIsLoading(false);
  }, []);

  useEffect(() => {
    loadSchedules().catch(() => {
      // Errors are already handled in loadSchedules
    });
    // Refresh schedules every 5 seconds
    const interval = setInterval(() => {
      loadSchedules().catch(() => {
        // Errors are already handled in loadSchedules
      });
    }, 5000);
    
    // Listen for download events to refresh schedules
    const unsubscribeDownloadStarted = EventsOn('download-started', () => {
      loadSchedules().catch(() => {
        // Errors are already handled in loadSchedules
      });
    });
    const unsubscribeDownloadCompleted = EventsOn('download-completed', () => {
      loadSchedules().catch(() => {
        // Errors are already handled in loadSchedules
      });
    });
    const unsubscribeFileSaved = EventsOn('file-saved', () => {
      loadSchedules().catch(() => {
        // Errors are already handled in loadSchedules
      });
    });
    
    return () => {
      clearInterval(interval);
      unsubscribeDownloadStarted();
      unsubscribeDownloadCompleted();
      unsubscribeFileSaved();
    };
  }, [loadSchedules]);

  const formatDateTime = (dateTimeStr: string): string => {
    if (dateTimeStr.length !== 14) {
      return dateTimeStr;
    }
    const year = dateTimeStr.substring(0, 4);
    const month = dateTimeStr.substring(4, 6);
    const day = dateTimeStr.substring(6, 8);
    const hour = dateTimeStr.substring(8, 10);
    const minute = dateTimeStr.substring(10, 12);
    return `${year}-${month}-${day} ${hour}:${minute}`;
  };

  const handleDeleteClick = (e: React.MouseEvent, program: radikron.Prog) => {
    e.stopPropagation();
    setProgramToDelete(program);
  };

  const handleDeleteConfirm = useCallback(async () => {
    if (!programToDelete) return;

    setIsDeleting(true);
    try {
      // @ts-ignore - DeleteManualInjection will be available after Wails rebuild
      await App.DeleteManualInjection(programToDelete.ID);
      setProgramToDelete(null);
      // Reload schedules to reflect the deletion
      await loadSchedules();
      // Only show success if reload succeeded
      const successMessage = `Program [${programToDelete.StationID}] ${programToDelete.Title} deleted successfully`;
      addActivityLog("success", successMessage);
      toast.success(successMessage);
    } catch (err) {
      console.error('Failed to delete manual injection:', err);
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(`Failed to delete program: ${errorMessage}`);
    } finally {
      setIsDeleting(false);
    }
  }, [programToDelete, loadSchedules, addActivityLog]);

  return (
    <div className="flex items-center justify-center px-4 py-8">
      <Card className="w-full max-w-6xl flex flex-col h-[600px]">
        <CardHeader className="flex-shrink-0">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Scheduled Downloads</CardTitle>
              <CardDescription>
                Programs queued for download based on matching rules
              </CardDescription>
            </div>
            <Button 
              onClick={() => {
                loadSchedules().catch(() => {
                  // Errors are already handled in loadSchedules
                });
              }} 
              disabled={isLoading} 
              variant="outline"
            >
              {isLoading ? 'Refreshing...' : 'Refresh'}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="flex-1 min-h-0 flex flex-col gap-4">
          {error && (
            <div className="flex-shrink-0 p-4 rounded-md bg-destructive/10 text-destructive border border-destructive/20">
              {error}
            </div>
          )}

          {!error && schedules.length > 0 && (
            <div className="flex-1 flex flex-col min-h-0">
              <div className="flex-shrink-0 text-sm text-muted-foreground mb-2">
                {schedules.length} scheduled program{schedules.length !== 1 ? 's' : ''}
              </div>
              <div className="flex-1 min-h-0 overflow-hidden">
                <ScrollArea className="h-full">
                  <div className="space-y-3 pr-4">
                    {schedules.map((program) => {
                      const isManual = program.IsManualInjection || false;
                      return (
                        <Card
                          key={program.ID}
                          className="p-4 cursor-pointer hover:bg-accent"
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
                          <div className="flex items-start justify-between">
                            <div className="flex-1">
                              <h3 className="font-semibold text-lg mb-1">{program.Title}</h3>
                              <div className="flex items-center gap-2 flex-wrap">
                                <Badge variant="outline">{program.StationID}</Badge>
                                {program.Pfm && (
                                  <span className="text-sm text-muted-foreground">
                                    Host: {program.Pfm}
                                  </span>
                                )}
                                {program.RuleName && (
                                  <Badge variant="secondary">Rule: {program.RuleName}</Badge>
                                )}
                                {isManual && (
                                  <Badge variant="outline" className="bg-blue-500/10 text-blue-500 border-blue-500/20">
                                    Manual
                                  </Badge>
                                )}
                              </div>
                              <div className="mt-2 text-sm text-muted-foreground">
                                <p>Start: {formatDateTime(program.Ft)}</p>
                                <p>End: {formatDateTime(program.To)}</p>
                              </div>
                            </div>
                            {isManual && (
                              <Button
                                variant="destructive"
                                size="sm"
                                onClick={(e) => handleDeleteClick(e, program)}
                                className="ml-2"
                              >
                                Delete
                              </Button>
                            )}
                          </div>
                        </Card>
                      );
                    })}
                  </div>
                </ScrollArea>
              </div>
            </div>
          )}

          {!error && !isLoading && schedules.length === 0 && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>No programs scheduled for download.</p>
              <p className="text-sm mt-2">Programs will appear here once they match your rules.</p>
            </div>
          )}

          {isLoading && schedules.length === 0 && (
            <div className="flex-shrink-0 text-center py-8 text-muted-foreground">
              <p>Loading scheduled downloads...</p>
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
                    {selectedProgram.RuleName && (
                      <Badge variant="secondary">Rule: {selectedProgram.RuleName}</Badge>
                    )}
                  </div>
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div>
                  <p className="text-sm font-medium text-muted-foreground mb-1">Time</p>
                  <p className="text-sm text-foreground">
                    {formatDateTime(selectedProgram.Ft)} - {formatDateTime(selectedProgram.To)}
                  </p>
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
                {selectedProgram.Tags && selectedProgram.Tags.length > 0 && (
                  <div>
                    <p className="text-sm font-medium text-muted-foreground mb-1">Tags</p>
                    <div className="flex flex-wrap gap-2">
                      {selectedProgram.Tags.map((tag, index) => (
                        <Badge key={index} variant="outline">
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={programToDelete !== null} onOpenChange={(open) => !open && setProgramToDelete(null)}>
        <DialogContent className="max-w-md [&>button]:text-foreground [&>button:hover]:text-foreground [&>button]:opacity-100 [&>button>svg]:text-foreground">
          <DialogHeader>
            <DialogTitle>Delete Manual Injection</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this manually injected program? This will remove it from scheduled downloads and the manual injection list.
            </DialogDescription>
          </DialogHeader>
          {programToDelete && (
            <div className="py-4">
              <p className="text-sm font-medium text-foreground mb-2">{programToDelete.Title}</p>
              <p className="text-sm text-muted-foreground">
                [{programToDelete.StationID}] {formatDateTime(programToDelete.Ft)}
              </p>
            </div>
          )}
          <div className="flex justify-end gap-2 mt-4">
            <Button
              variant="outline"
              onClick={() => setProgramToDelete(null)}
              disabled={isDeleting}
              className="border-foreground/20 text-foreground hover:bg-accent hover:text-accent-foreground"
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteConfirm}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
};

