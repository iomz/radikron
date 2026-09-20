import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../wailsjs/go/main/App';
import { useAppStore } from './useAppStore';

vi.mock('../../wailsjs/go/main/App', () => ({
  GetAvailableStations: vi.fn(),
  GetConfig: vi.fn(),
  GetMonitoringStatus: vi.fn(),
  LoadConfig: vi.fn(),
  StartMonitoring: vi.fn(),
  StopMonitoring: vi.fn(),
}));

const backend = vi.mocked(Backend);

function resetStore() {
  useAppStore.setState({
    monitoring: false,
    configInfo: null,
    stations: [],
    configFile: 'config.yml',
    activityLogs: [],
    loading: true,
    isToggling: false,
  });
}

describe('useAppStore', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    resetStore();
  });

  it('loads initial state through generated backend calls', async () => {
    const cfg = { AreaID: 'JP13' } as Awaited<ReturnType<typeof Backend.GetConfig>>;
    backend.GetConfig.mockResolvedValue(cfg);
    backend.GetAvailableStations.mockResolvedValue(['TBS', 'FMT']);
    backend.GetMonitoringStatus.mockResolvedValue(true);

    await useAppStore.getState().loadInitialData();

    expect(backend.GetConfig).toHaveBeenCalledOnce();
    expect(backend.GetAvailableStations).toHaveBeenCalledOnce();
    expect(backend.GetMonitoringStatus).toHaveBeenCalledOnce();
    expect(useAppStore.getState()).toMatchObject({
      configInfo: cfg,
      stations: ['TBS', 'FMT'],
      monitoring: true,
      loading: false,
    });
  });

  it('serializes monitoring toggles and trusts verified backend state', async () => {
    let releaseStart: (() => void) | undefined;
    backend.StartMonitoring.mockImplementation(() => new Promise<void>((resolve) => {
      releaseStart = resolve;
    }));
    backend.GetMonitoringStatus.mockResolvedValue(true);

    const firstToggle = useAppStore.getState().toggleMonitoring();
    const secondToggle = useAppStore.getState().toggleMonitoring();

    expect(backend.StartMonitoring).toHaveBeenCalledOnce();
    expect(useAppStore.getState().isToggling).toBe(true);
    releaseStart?.();
    await Promise.all([firstToggle, secondToggle]);

    expect(backend.GetMonitoringStatus).toHaveBeenCalledOnce();
    expect(useAppStore.getState()).toMatchObject({ monitoring: true, isToggling: false });
  });

  it('records backend toggle errors and clears guard', async () => {
    backend.StartMonitoring.mockRejectedValue(new Error('backend unavailable'));

    await useAppStore.getState().toggleMonitoring();

    expect(useAppStore.getState().monitoring).toBe(false);
    expect(useAppStore.getState().isToggling).toBe(false);
    const logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]).toMatchObject({
      type: 'error',
      message: 'Failed to start monitoring: backend unavailable',
    });
  });

  it('refreshes configuration and stations after loading a file', async () => {
    const cfg = { AreaID: 'JP27' } as Awaited<ReturnType<typeof Backend.GetConfig>>;
    backend.LoadConfig.mockResolvedValue(undefined);
    backend.GetConfig.mockResolvedValue(cfg);
    backend.GetAvailableStations.mockResolvedValue(['FM802']);

    await useAppStore.getState().loadConfig('/tmp/radikron.yml');

    expect(backend.LoadConfig).toHaveBeenCalledWith('/tmp/radikron.yml');
    expect(useAppStore.getState()).toMatchObject({ configInfo: cfg, stations: ['FM802'] });
  });
});
