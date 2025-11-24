import React, { useEffect, useState } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Dashboard } from "@/components/Dashboard";
import { RulesEditor } from "@/components/RulesEditor";
import { ScheduledDownloads } from "@/components/ScheduledDownloads";
import { ProgramSearchBrowser } from "@/components/ProgramSearchBrowser";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useAppStore } from "@/store/useAppStore";
import { useThemeStore } from "@/store/useThemeStore";
import iconBlack from "./assets/black.png";
import iconWhite from "./assets/white.png";

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
  const monitoring = useAppStore((state) => state.monitoring);
  const loading = useAppStore((state) => state.loading);
  const loadInitialData = useAppStore((state) => state.loadInitialData);
  const toggleMonitoring = useAppStore((state) => state.toggleMonitoring);
  const setMonitoring = useAppStore((state) => state.setMonitoring);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const getEffectiveTheme = useThemeStore((state) => state.getEffectiveTheme);
  const theme = useThemeStore((state) => state.theme);
  const [effectiveTheme, setEffectiveTheme] = useState<"light" | "dark">(() =>
    getEffectiveTheme(),
  );

  // Update effective theme when theme changes
  useEffect(() => {
    setEffectiveTheme(getEffectiveTheme());

    // Listen for system theme changes if theme is 'system'
    if (theme === "system" && typeof window !== "undefined") {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      const handleChange = () => {
        setEffectiveTheme(getEffectiveTheme());
      };

      if (mediaQuery.addEventListener) {
        mediaQuery.addEventListener("change", handleChange);
        return () => mediaQuery.removeEventListener("change", handleChange);
      } else {
        mediaQuery.addListener(handleChange);
        return () => mediaQuery.removeListener(handleChange);
      }
    }
  }, [theme, getEffectiveTheme]);

  const appIcon = effectiveTheme === "dark" ? iconBlack : iconWhite;

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, [loadInitialData]);

  // Set up event listeners
  useEffect(() => {
    // Listen for monitoring status changes
    const unsubscribeStarted = EventsOn("monitoring-started", () => {
      setMonitoring(true);
      addActivityLog("success", "Monitoring started");
      console.log("Monitoring started");
    });

    const unsubscribeStopped = EventsOn("monitoring-stopped", () => {
      setMonitoring(false);
      addActivityLog("info", "Monitoring stopped");
      console.log("Monitoring stopped");
    });

    // Listen for download events
    const unsubscribeDownloadStarted = EventsOn(
      "download-started",
      (data: DownloadEventData) => {
        addActivityLog(
          "info",
          `Started downloading: ${data.title} (${data.station})`,
        );
      },
    );

    const unsubscribeDownloadCompleted = EventsOn(
      "download-completed",
      (data: DownloadEventData) => {
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

    const unsubscribeDownloadFailed = EventsOn(
      "download-failed",
      (data: DownloadEventData) => {
        addActivityLog(
          "error",
          `Failed: ${data.title} (${data.station}) - ${data.error || "Unknown error"}`,
        );
      },
    );

    const unsubscribeConfigLoaded = EventsOn(
      "config-loaded",
      (data: ConfigLoadedData) => {
        if (data.success) {
          addActivityLog("success", "Configuration loaded successfully");
          loadConfigInfo();
        }
      },
    );

    // Listen for log messages from radikron
    const unsubscribeLogMessage = EventsOn(
      "log-message",
      (data: LogMessageData) => {
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
      unsubscribeStarted();
      unsubscribeStopped();
      unsubscribeDownloadStarted();
      unsubscribeDownloadCompleted();
      unsubscribeDownloadFailed();
      unsubscribeConfigLoaded();
      unsubscribeLogMessage();
    };
  }, [setMonitoring, addActivityLog, loadConfigInfo]);

  return (
    <ErrorBoundary FallbackComponent={ErrorFallback}>
      {loading ? (
        <div className="flex items-center justify-center min-h-screen bg-background">
          <p className="text-muted-foreground">Loading...</p>
        </div>
      ) : (
        <Tabs defaultValue="dashboard" className="min-h-screen bg-background text-foreground flex flex-col">
          <header className="border-b bg-card">
            <div className="container mx-auto px-4 py-4">
              <div className="flex items-center justify-between mb-4">

                <div className="flex items-center gap-3">
                  <img src={appIcon} alt="Radikron" className="w-8 h-8" />
                  <h1 className="text-2xl font-bold">Radikron</h1>
                </div>

                <TabsList>
                  <TabsTrigger value="dashboard">Dashboard</TabsTrigger>
                  <TabsTrigger value="rules">Rules Editor</TabsTrigger>
                  <TabsTrigger value="scheduled">Scheduled Downloads</TabsTrigger>
                  <TabsTrigger value="programs">Program Search</TabsTrigger>
                </TabsList>

                <div className="flex items-center gap-4">
                  <div>
                    <Badge variant={monitoring ? "default" : "secondary"}>
                      {monitoring && (
                        <span
                          // Positioning and styling using Tailwind classes
                          className="h-2 w-2 bg-red-500 rounded-full border border-white dark:border-gray-900 animate-pulse-grow"
                        />
                      )}
                      {monitoring ? "Running" : "Stopped"}
                    </Badge>
                  </div>
                  <Button onClick={toggleMonitoring}>
                    {monitoring ? "Stop Monitoring" : "Start Monitoring"}
                  </Button>
                  <ThemeToggle />
                </div>

              </div>
            </div>
          </header>

          <main className="flex-1 px-4 py-4">
            <div className="container mx-auto">
              <TabsContent value="dashboard">
                <Dashboard />
              </TabsContent>
              <TabsContent value="rules">
                <RulesEditor />
              </TabsContent>
              <TabsContent value="scheduled">
                <ScheduledDownloads />
              </TabsContent>
              <TabsContent value="programs">
                <ProgramSearchBrowser />
              </TabsContent>
            </div>
          </main>
        </Tabs>
      )}
    </ErrorBoundary>
  );
};

export default AppComponent;
