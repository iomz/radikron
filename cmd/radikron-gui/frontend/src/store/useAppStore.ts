import { create } from 'zustand';
import { config } from '../../wailsjs/go/models';
import * as App from '../../wailsjs/go/main/App';

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
  configFile: string;
  activityLogs: ActivityLogEntry[];
  loading: boolean;

  // Actions
  setConfigInfo: (configInfo: config.Config | null) => void;
  setStations: (stations: string[]) => void;
  setConfigFile: (configFile: string) => void;
  addActivityLog: (type: 'info' | 'success' | 'error', message: string) => void;
  setLoading: (loading: boolean) => void;

  // Async actions
  loadConfigInfo: () => Promise<void>;
  loadStations: () => Promise<void>;
  loadInitialData: () => Promise<void>;
  loadConfig: (filename: string) => Promise<void>;
  refreshStations: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  // Initial state
  configInfo: null,
  stations: [],
  configFile: 'config.yml',
  activityLogs: [],
  loading: true,

  // Synchronous actions
  setConfigInfo: (configInfo) => set({ configInfo }),
  setStations: (stations) => set({ stations }),
  setConfigFile: (configFile) => set({ configFile }),
  setLoading: (loading) => set({ loading }),

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
      set({ stations: Array.isArray(stationList) ? stationList : [] });
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
      const stations = Array.isArray(stationList) ? stationList : [];
      set({ stations });
      get().addActivityLog('success', `Loaded ${stations.length} stations`);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to refresh stations: ${errorMessage}`);
    }
  },
}));
