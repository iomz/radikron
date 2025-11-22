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
  monitoring: boolean;
  configInfo: config.Config | null;
  stations: string[];
  configFile: string;
  activityLogs: ActivityLogEntry[];
  loading: boolean;

  // Actions
  setMonitoring: (monitoring: boolean) => void;
  setConfigInfo: (configInfo: config.Config | null) => void;
  setStations: (stations: string[]) => void;
  setConfigFile: (configFile: string) => void;
  addActivityLog: (type: 'info' | 'success' | 'error', message: string) => void;
  setLoading: (loading: boolean) => void;

  // Async actions
  loadConfigInfo: () => Promise<void>;
  loadStations: () => Promise<void>;
  loadMonitoringStatus: () => Promise<void>;
  loadInitialData: () => Promise<void>;
  toggleMonitoring: () => Promise<void>;
  loadConfig: (filename: string) => Promise<void>;
  refreshStations: () => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  // Initial state
  monitoring: false,
  configInfo: null,
  stations: [],
  configFile: 'config.yml',
  activityLogs: [],
  loading: true,

  // Synchronous actions
  setMonitoring: (monitoring) => set({ monitoring }),
  setConfigInfo: (configInfo) => set({ configInfo }),
  setStations: (stations) => set({ stations }),
  setConfigFile: (configFile) => set({ configFile }),
  setLoading: (loading) => set({ loading }),

  addActivityLog: (type, message) => {
    const entry: ActivityLogEntry = {
      id: Date.now(),
      type,
      message,
      timestamp: new Date().toLocaleTimeString(),
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
      set({ stations: stationList });
    } catch (error) {
      console.error('Failed to load stations:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to load stations: ${errorMessage}`);
    }
  },

  loadMonitoringStatus: async () => {
    try {
      const status = await App.GetMonitoringStatus();
      set({ monitoring: status });
    } catch (error) {
      console.error('Failed to load monitoring status:', error);
    }
  },

  loadInitialData: async () => {
    set({ loading: true });
    try {
      await Promise.all([
        get().loadConfigInfo(),
        get().loadStations(),
        get().loadMonitoringStatus(),
      ]);
    } finally {
      set({ loading: false });
    }
  },

  toggleMonitoring: async () => {
    const { monitoring } = get();
    try {
      if (monitoring) {
        await App.StopMonitoring();
      } else {
        await App.StartMonitoring();
      }
    } catch (error) {
      console.error('Failed to toggle monitoring:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to ${monitoring ? 'stop' : 'start'} monitoring: ${errorMessage}`);
    }
  },

  loadConfig: async (filename: string) => {
    try {
      await App.LoadConfig(filename);
    } catch (error) {
      console.error('Failed to load config:', error);
      const errorMessage = error instanceof Error ? error.message : String(error);
      get().addActivityLog('error', `Failed to load config: ${errorMessage}`);
    }
  },

  refreshStations: async () => {
    await get().loadStations();
  },
}));

