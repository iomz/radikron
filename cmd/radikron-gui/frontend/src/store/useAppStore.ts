import { create } from 'zustand';
import * as config from '../../bindings/github.com/iomz/radikron/internal/config';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Downloads, Status } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

interface ActivityLogEntry {
  id: number;
  type: 'info' | 'success' | 'error';
  message: string;
  timestamp: string;
}

interface AppState {
  // State
  configInfo: config.Config | null;
  stations: string[];
  stationNames: Record<string, string>;
  configFile: string;
  activityLogs: ActivityLogEntry[];
  loading: boolean;
  // Backend status: automatic checks and failures that need attention.
  status: Status | null;
  // Scheduled programs with their download state, and this session's results.
  downloads: Downloads | null;

  // Actions
  setConfigInfo: (configInfo: config.Config | null) => void;
  setStations: (stations: string[]) => void;
  stationLabel: (stationID: string) => string;
  setConfigFile: (configFile: string) => void;
  addActivityLog: (type: 'info' | 'success' | 'error', message: string) => void;
  setLoading: (loading: boolean) => void;
  setStatus: (status: Status) => void;

  // Async actions
  loadConfigInfo: () => Promise<void>;
  loadStations: () => Promise<void>;
  loadInitialData: () => Promise<void>;
  loadConfig: (filename: string) => Promise<void>;
  refreshStations: () => Promise<void>;
  loadStatus: () => Promise<void>;
  loadDownloads: () => Promise<void>;
  // Retries what failed (startup, config reload, station catalog) and reloads
  // the data that depends on it.
  retry: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  // Initial state
  configInfo: null,
  stations: [],
  stationNames: {},
  configFile: 'config.yml',
  activityLogs: [],
  loading: true,
  status: null,
  downloads: null,

  // Synchronous actions
  setConfigInfo: (configInfo) => set({ configInfo }),
  setStations: (stations) => set({ stations }),
  stationLabel: (stationID) => get().stationNames[stationID] || stationID,
  setConfigFile: (configFile) => set({ configFile }),
  setLoading: (loading) => set({ loading }),
  setStatus: (status) => set({ status }),

  addActivityLog: (type, message) => {
    const now = new Date();
    const entry: ActivityLogEntry = {
      id: Date.now(),
      type,
      message,
      timestamp: now.toISOString(), // Store full date/time as ISO string for consistent formatting
    };
    set((state) => {
      const newLogs = [...state.activityLogs, entry];
      // Keep only last 50 entries
      return { activityLogs: newLogs.slice(-50) };
    });
  },

  // Async actions
  loadConfigInfo: async () => {
    try {
      const cfg = await App.GetConfig();
      set({ configInfo: cfg });
    } catch (error) {
      console.error('Failed to load config:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to load config: ${errorMessage}`);
    }
  },

  loadStations: async () => {
    try {
      const stationList = await App.GetAvailableStations();
      const stationNames = await App.GetStationNames();
      set({
        stations: Array.isArray(stationList) ? stationList : [],
        stationNames: (stationNames || {}) as Record<string, string>,
      });
    } catch (error) {
      console.error('Failed to load stations:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to load stations: ${errorMessage}`);
    }
  },

  loadInitialData: async () => {
    set({ loading: true });
    try {
      await Promise.all([
        get().loadConfigInfo(),
        get().loadStations(),
        get().loadStatus(),
        get().loadDownloads(),
      ]);
    } finally {
      set({ loading: false });
    }
  },

  loadConfig: async (filename: string) => {
    try {
      await App.LoadConfig(filename);
      // Refresh derived data after a successful load
      await Promise.all([
        get().loadConfigInfo(),
        get().loadStations(),
      ]);
    } catch (error) {
      console.error('Failed to load config:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to load config: ${errorMessage}`);
    }
  },

  refreshStations: async () => {
    try {
      const stationList = await App.RefreshStations();
      const stationNames = await App.GetStationNames();
      const stations = Array.isArray(stationList) ? stationList : [];
      set({ stations, stationNames: (stationNames || {}) as Record<string, string> });
      get().addActivityLog('success', `Loaded ${stations.length} stations`);
      // A successful refresh clears a station error shown in the status.
      await get().loadStatus();
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to refresh stations: ${errorMessage}`);
      await get().loadStatus();
    }
  },

  loadStatus: async () => {
    try {
      set({ status: await App.GetStatus() });
    } catch (error) {
      console.error('Failed to load status:', error);
    }
  },

  loadDownloads: async () => {
    try {
      set({ downloads: await App.GetDownloads() });
    } catch (error) {
      // Startup failures are reported through the status instead.
      console.error('Failed to load downloads:', error);
    }
  },

  retry: async () => {
    try {
      await App.Retry();
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Retry failed: ${errorMessage}`);
    }
    // Reload whatever recovered; failures stay visible in the status.
    await Promise.all([get().loadConfigInfo(), get().loadStations(), get().loadStatus(), get().loadDownloads()]);
  },
}));
