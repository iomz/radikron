import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../wailsjs/go/main/App';
import { useAppStore } from './useAppStore';

vi.mock('../../wailsjs/go/main/App', () => ({
  GetAvailableStations: vi.fn(),
  GetConfig: vi.fn(),
  LoadConfig: vi.fn(),
  RefreshStations: vi.fn(),
}));

const backend = vi.mocked(Backend);

function resetStore() {
  useAppStore.setState({
    configInfo: null,
    stations: [],
    configFile: 'config.yml',
    activityLogs: [],
    loading: true,
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
    await useAppStore.getState().loadInitialData();

    expect(backend.GetConfig).toHaveBeenCalledOnce();
    expect(backend.GetAvailableStations).toHaveBeenCalledOnce();
    expect(useAppStore.getState()).toMatchObject({
      configInfo: cfg,
      stations: ['TBS', 'FMT'],
      loading: false,
    });
  });

  it('normalizes a null station response to an empty list', async () => {
    backend.GetAvailableStations.mockResolvedValue(null as unknown as string[]);

    await useAppStore.getState().loadStations();

    expect(useAppStore.getState().stations).toEqual([]);
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

  it('retries station catalog refresh and updates visible stations', async () => {
    backend.RefreshStations.mockResolvedValue(['TBS', 'QRR']);

    await useAppStore.getState().refreshStations();

    expect(backend.RefreshStations).toHaveBeenCalledOnce();
    expect(useAppStore.getState().stations).toEqual(['TBS', 'QRR']);
  });
});
