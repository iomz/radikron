import React, { useEffect } from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Configuration } from '@/components/Configuration';
import { Stations } from '@/components/Stations';
import { Activity } from '@/components/Activity';
import { ThemeToggle } from '@/components/ThemeToggle';
import { useAppStore } from '@/store/useAppStore';

// Type definitions for event data
interface DownloadEventData {
  station: string;
  title: string;
  start?: string;
  error?: string;
}

interface ConfigLoadedData {
  success: boolean;
}

const AppComponent: React.FC = () => {
  const monitoring = useAppStore((state) => state.monitoring);
  const loading = useAppStore((state) => state.loading);
  const loadInitialData = useAppStore((state) => state.loadInitialData);
  const toggleMonitoring = useAppStore((state) => state.toggleMonitoring);
  const setMonitoring = useAppStore((state) => state.setMonitoring);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, [loadInitialData]);

  // Set up event listeners
  useEffect(() => {
    // Listen for monitoring status changes
    const unsubscribeStarted = EventsOn('monitoring-started', () => {
      setMonitoring(true);
      addActivityLog('success', 'Monitoring started');
      console.log('Monitoring started');
    });

    const unsubscribeStopped = EventsOn('monitoring-stopped', () => {
      setMonitoring(false);
      addActivityLog('info', 'Monitoring stopped');
      console.log('Monitoring stopped');
    });

    // Listen for download events
    const unsubscribeDownloadStarted = EventsOn('download-started', (data: DownloadEventData) => {
      addActivityLog('info', `Started downloading: ${data.title} (${data.station})`);
    });

    const unsubscribeDownloadCompleted = EventsOn('download-completed', (data: DownloadEventData) => {
      addActivityLog('success', `Completed: ${data.title} (${data.station})`);
    });

    const unsubscribeDownloadFailed = EventsOn('download-failed', (data: DownloadEventData) => {
      addActivityLog('error', `Failed: ${data.title} (${data.station}) - ${data.error || 'Unknown error'}`);
    });

    const unsubscribeConfigLoaded = EventsOn('config-loaded', (data: ConfigLoadedData) => {
      if (data.success) {
        addActivityLog('success', 'Configuration loaded successfully');
        loadConfigInfo();
      }
    });

    // Listen for log messages from radikron
    const unsubscribeLogMessage = EventsOn('log-message', (data: { type: 'info' | 'success' | 'error'; message: string }) => {
      addActivityLog(data.type, data.message);
    });

    // Cleanup
    return () => {
      unsubscribeStarted();
      unsubscribeStopped();
      unsubscribeDownloadStarted();
      unsubscribeDownloadCompleted();
      unsubscribeDownloadFailed();
      unsubscribeConfigLoaded();
      unsubscribeLogMessage();
    };
  }, [setMonitoring, addActivityLog, loadConfigInfo]);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      <header className="border-b bg-card">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <h1 className="text-2xl font-bold">Radikron</h1>
          <div className="flex items-center gap-4">
            <Badge variant={monitoring ? 'default' : 'secondary'}>
              {monitoring ? 'Running' : 'Stopped'}
            </Badge>
            <Button onClick={toggleMonitoring}>
              {monitoring ? 'Stop Monitoring' : 'Start Monitoring'}
            </Button>
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="flex-1 flex items-center justify-center px-4 py-8">
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-2 w-fit max-w-7xl">
          <Configuration />
          <Stations />
          <Activity />
        </div>
      </main>
    </div>
  );
};

export default AppComponent;
