import React, { useEffect, useState } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Overview } from "@/components/screens/Overview";
import { Downloads } from "@/components/screens/Downloads";
import { Rules } from "@/components/screens/Rules";
import { Search } from "@/components/screens/Search";
import { Settings } from "@/components/Settings";
import { Timetable } from "@/components/Timetable";
import { AppShell } from "@/components/shell/AppShell";
import type { ScreenId } from "@/lib/navigation";
import { useAppStore } from "@/store/useAppStore";
import type { Status } from "../bindings/github.com/iomz/radikron/cmd/radikron-gui/models";

// Type definitions for event data
interface DownloadEventData {
  station: string;
  title: string;
  start?: string;
  error?: string;
  stage?: string;
  completed?: number;
  total?: number;
}

interface ConfigLoadedData {
  success: boolean;
}

interface LogMessageData {
  type: "info" | "success" | "error";
  message: string;
}

// Error fallback component
const ErrorFallback: React.FC<{
  error: Error;
  resetErrorBoundary: () => void;
}> = ({ error, resetErrorBoundary }) => {
  return (
    <div className="flex items-center justify-center min-h-screen bg-background p-4">
      <div className="max-w-md w-full bg-card border border-destructive rounded-lg p-6 space-y-4">
        <h2 className="text-xl font-bold text-destructive">
          Something went wrong
        </h2>
        <p className="text-muted-foreground">
          An unexpected error occurred. Please try again or restart the
          application.
        </p>
        <details className="text-sm">
          <summary className="cursor-pointer text-muted-foreground hover:text-foreground mb-2">
            Error details
          </summary>
          <pre className="mt-2 p-3 bg-muted rounded text-xs text-foreground overflow-auto">
            {error.message}
          </pre>
        </details>
        <Button onClick={resetErrorBoundary} className="w-full">
          Try again
        </Button>
      </div>
    </div>
  );
};

const AppComponent: React.FC = () => {
  const loading = useAppStore((state) => state.loading);
  const loadInitialData = useAppStore((state) => state.loadInitialData);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const loadStations = useAppStore((state) => state.loadStations);
  const setStatus = useAppStore((state) => state.setStatus);
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const [screen, setScreen] = useState<ScreenId>("overview");

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, [loadInitialData]);

  // Set up event listeners
  useEffect(() => {
    // Listen for download events
    const unsubscribeDownloadStarted = Events.On(
      "download-started",
      ({ data }: { data: DownloadEventData }) => {
        addActivityLog(
          "info",
          `Started downloading: ${data.title} (${data.station})`,
        );
      },
    );

    const unsubscribeDownloadCompleted = Events.On(
      "download-completed",
      ({ data }: { data: DownloadEventData }) => {
        let dateStr = "";
        if (data.start && data.start.length === 14) {
          // Format: YYYYMMDDHHmmss -> YYYY/MM/DD
          const year = data.start.substring(0, 4);
          const month = data.start.substring(4, 6);
          const day = data.start.substring(6, 8);
          const hour = data.start.substring(8, 10);
          const minute = data.start.substring(10, 12);
          const second = data.start.substring(12, 14);
          dateStr = `${year}-${month}-${day} ${hour}:${minute}:${second}`;
        }
        const message = dateStr
          ? `Completed: [${data.station}]${data.title} - ${dateStr}`
          : `Completed: [${data.station}]${data.title}`;
        addActivityLog("success", message);
        toast.success(message);
      },
    );

    const unsubscribeDownloadFailed = Events.On(
      "download-failed",
      ({ data }: { data: DownloadEventData }) => {
        addActivityLog(
          "error",
          `Failed ${data.stage ? `during ${data.stage} ` : ""}: ${data.title} (${data.station}) - ${data.error || "Unknown error"}`,
        );
      },
    );

    const unsubscribeDownloadProgress = Events.On(
      "download-progress",
      ({ data }: { data: DownloadEventData }) => {
        const progress = data.total
          ? ` (${data.completed}/${data.total})`
          : "";
        addActivityLog(
          "info",
          `Download ${data.stage || "in progress"}${progress}: ${data.title} (${data.station})`,
        );
      },
    );

    const unsubscribeStationsLoaded = Events.On("stations-loaded", () => {
      void loadStations();
    });

    const unsubscribeDownloadsChanged = Events.On("downloads-changed", () => {
      void loadDownloads();
    });

    const unsubscribeStatusChanged = Events.On(
      "status-changed",
      ({ data }: { data: Status }) => {
        setStatus(data);
      },
    );

    const unsubscribeConfigLoaded = Events.On(
      "config-loaded",
      ({ data }: { data: ConfigLoadedData }) => {
        if (data.success) {
          addActivityLog("success", "Configuration loaded successfully");
          loadConfigInfo();
        }
      },
    );

    // Listen for log messages from radikron
    const unsubscribeLogMessage = Events.On(
      "log-message",
      ({ data }: { data: LogMessageData }) => {
        // Filter out duplicate messages that are already handled by structured events
        const message = data.message.toLowerCase();
        // Skip messages that duplicate structured events
        if (
          message.includes("start downloading") ||
          message.includes("download completed") ||
          message.includes("+file saved") ||
          message.includes("start encoding to mp3") ||
          message.includes("finish encoding to mp3")
        ) {
          return; // Skip duplicate log messages
        }
        addActivityLog(data.type, data.message);
      },
    );

    // Cleanup
    return () => {
      unsubscribeDownloadStarted();
      unsubscribeDownloadCompleted();
      unsubscribeDownloadFailed();
      unsubscribeDownloadProgress();
      unsubscribeStationsLoaded();
      unsubscribeStatusChanged();
      unsubscribeDownloadsChanged();
      unsubscribeConfigLoaded();
      unsubscribeLogMessage();
    };
  }, [addActivityLog, loadConfigInfo, loadStations, setStatus, loadDownloads]);

  return (
    <ErrorBoundary FallbackComponent={ErrorFallback}>
      {loading ? (
        <div className="flex items-center justify-center min-h-screen bg-background">
          <p className="text-muted-foreground">Loading...</p>
        </div>
      ) : (
        <AppShell screen={screen} onNavigate={setScreen}>
          {screen === "overview" && <Overview onNavigate={setScreen} />}
          {screen === "downloads" && <Downloads />}
          {screen === "timetable" && <Timetable />}
          {screen === "search" && <Search />}
          {screen === "rules" && <Rules onNavigate={setScreen} />}
          {screen === "settings" && <Settings />}
        </AppShell>
      )}
    </ErrorBoundary>
  );
};

export default AppComponent;
