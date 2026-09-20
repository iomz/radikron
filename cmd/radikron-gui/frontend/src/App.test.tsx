import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import App from './App';
import { useAppStore } from './store/useAppStore';

vi.mock('../wailsjs/go/main/App', () => ({
  GetAvailableStations: vi.fn(),
  GetConfig: vi.fn(),
  GetMonitoringStatus: vi.fn(),
  StartMonitoring: vi.fn(),
  StopMonitoring: vi.fn(),
}));

vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn() } }));
vi.mock('@/components/Dashboard', () => ({ Dashboard: () => <div>Dashboard content</div> }));
vi.mock('@/components/RulesEditor', () => ({ RulesEditor: () => <div>Rules content</div> }));
vi.mock('@/components/ScheduledDownloads', () => ({ ScheduledDownloads: () => <div>Schedules content</div> }));
vi.mock('@/components/ProgramSearchBrowser', () => ({ ProgramSearchBrowser: () => <div>Programs content</div> }));
vi.mock('@/components/ThemeToggle', () => ({ ThemeToggle: () => <button>Theme</button> }));

const backend = vi.mocked(Backend);
const eventsOn = vi.mocked(EventsOn);
let eventHandlers: Map<string, (data?: unknown) => void>;

describe('App event transitions', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.spyOn(console, 'log').mockImplementation(() => undefined);
    eventHandlers = new Map();
    eventsOn.mockImplementation((name, callback) => {
      eventHandlers.set(name, callback as (data?: unknown) => void);
      return vi.fn();
    });
    backend.GetConfig.mockResolvedValue({ AreaID: 'JP13' } as Awaited<ReturnType<typeof Backend.GetConfig>>);
    backend.GetAvailableStations.mockResolvedValue(['TBS']);
    backend.GetMonitoringStatus.mockResolvedValue(false);
    useAppStore.setState({
      monitoring: false,
      configInfo: null,
      stations: [],
      activityLogs: [],
      loading: true,
      isToggling: false,
    });
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
  });

  it('updates monitoring state from backend events and unsubscribes on unmount', async () => {
    const { unmount } = render(<App />);
    await screen.findByText('Stopped');
    expect(screen.getByText('Dashboard content')).toBeTruthy();

    fireEvent.click(screen.getByRole('tab', { name: 'Rules Editor' }));
    expect(screen.getByText('Rules content')).toBeTruthy();
    expect(screen.queryByText('Dashboard content')).toBeNull();

    act(() => eventHandlers.get('monitoring-started')?.());
    expect(screen.getByText('Running')).toBeTruthy();
    let logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toBe('Monitoring started');

    act(() => eventHandlers.get('monitoring-stopped')?.());
    expect(screen.getByText('Stopped')).toBeTruthy();
    logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toBe('Monitoring stopped');

    const unsubscribers = eventsOn.mock.results.map((result) => result.value);
    unmount();
    expect(unsubscribers).toHaveLength(7);
    unsubscribers.forEach((unsubscribe) => expect(unsubscribe).toHaveBeenCalledOnce());
  });

  it('refreshes config after successful config-loaded event', async () => {
    render(<App />);
    await screen.findByText('Stopped');
    backend.GetConfig.mockClear();

    act(() => eventHandlers.get('config-loaded')?.({ success: true }));

    await waitFor(() => expect(backend.GetConfig).toHaveBeenCalledOnce());
    const logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toBe('Configuration loaded successfully');
  });
});
