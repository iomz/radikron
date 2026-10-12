import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { Events } from '@wailsio/runtime';
import App from './App';
import { useAppStore } from './store/useAppStore';

vi.mock('../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  GetAvailableStations: vi.fn(),
  GetConfig: vi.fn(),
  GetStationNames: vi.fn(),
  GetStatus: vi.fn(),
  GetDownloads: vi.fn(),
}));

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn() } }));
vi.mock('@/components/screens/Overview', () => ({ Overview: () => <div>Dashboard content</div> }));
vi.mock('@/components/screens/Rules', () => ({ Rules: () => <div>Rules content</div> }));
vi.mock('@/components/screens/Downloads', () => ({ Downloads: () => <div>Schedules content</div> }));
vi.mock('@/components/screens/Search', () => ({ Search: () => <div>Programs content</div> }));
vi.mock('@/components/ThemeToggle', () => ({ ThemeToggle: () => <button>Theme</button> }));
vi.mock('@/components/screens/Settings', () => ({ Settings: () => <div>Settings content</div> }));
vi.mock('@/components/Timetable', () => ({ Timetable: () => <div>Timetable content</div> }));
vi.mock('@/components/UpdateButton', () => ({ UpdateButton: () => null }));

const backend = vi.mocked(Backend);
const eventsOn = vi.mocked(Events.On);
let eventHandlers: Map<string, (data?: unknown) => void>;

describe('App shell events', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.spyOn(console, 'log').mockImplementation(() => undefined);
    eventHandlers = new Map();
    eventsOn.mockImplementation((name, callback) => {
      // Wails v3 delivers a WailsEvent; tests pass the payload as its data.
      const handler = callback as (event: { data?: unknown }) => void;
      eventHandlers.set(name, (data?: unknown) => handler({ data }));
      return vi.fn();
    });
    backend.GetConfig.mockResolvedValue({ AreaID: 'JP13' } as Awaited<ReturnType<typeof Backend.GetConfig>>);
    backend.GetAvailableStations.mockResolvedValue(['TBS']);
    backend.GetStationNames.mockResolvedValue({ TBS: 'Tokyo Broadcasting System' });
    backend.GetStatus.mockResolvedValue({ monitoring: true } as Awaited<ReturnType<typeof Backend.GetStatus>>);
    backend.GetDownloads.mockResolvedValue({ scheduled: [], recent: [], skipped: [] } as Awaited<ReturnType<typeof Backend.GetDownloads>>);
    useAppStore.setState({
      configInfo: null,
      stations: [],
      stationNames: {},
      activityLogs: [],
      loading: true,
      status: null,
      downloads: null,
    });
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
  });

  it('omits monitoring controls and unsubscribes on unmount', async () => {
    const { unmount } = render(<App />);
    await screen.findByText('Dashboard content');
    expect(screen.getByText('Dashboard content')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /start monitoring|stop monitoring/i })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Rules' }));
    expect(screen.getByText('Rules content')).toBeTruthy();
    expect(screen.queryByText('Dashboard content')).toBeNull();
    expect(screen.getByRole('button', { name: 'Rules' }).getAttribute('aria-current')).toBe('page');
    fireEvent.click(screen.getByRole('button', { name: 'Timetable' }));
    expect(screen.getByText('Timetable content')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Settings' }));
    expect(screen.getByText('Settings content')).toBeTruthy();

    act(() => eventHandlers.get('download-progress')?.({
      station: 'TBS',
      title: 'Show',
      stage: 'fetching playlist',
      completed: 5,
      total: 10,
    }));
    const logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toContain('Download fetching playlist (5/10): Show (TBS)');

    act(() => eventHandlers.get('status-changed')?.({ monitoring: true, configError: 'bad config' }));
    expect(useAppStore.getState().status?.configError).toBe('bad config');

    backend.GetDownloads.mockClear();
    act(() => eventHandlers.get('downloads-changed')?.());
    await waitFor(() => expect(backend.GetDownloads).toHaveBeenCalledOnce());

    const unsubscribers = eventsOn.mock.results.map((result) => result.value);
    unmount();
    expect(unsubscribers).toHaveLength(9);
    unsubscribers.forEach((unsubscribe) => expect(unsubscribe).toHaveBeenCalledOnce());
  });

  it('refreshes config after successful config-loaded event', async () => {
    render(<App />);
    await screen.findByText('Dashboard content');
    backend.GetConfig.mockClear();

    act(() => eventHandlers.get('config-loaded')?.({ success: true }));

    await waitFor(() => expect(backend.GetConfig).toHaveBeenCalledOnce());
    const logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toBe('Configuration loaded successfully');
  });
});
