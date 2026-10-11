import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from './useAppStore';

vi.mock('../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  GetAvailableStations: vi.fn(),
  GetConfig: vi.fn(),
  GetStationNames: vi.fn(),
  GetStatus: vi.fn(),
  GetDownloads: vi.fn(),
  LoadConfig: vi.fn(),
  RefreshStations: vi.fn(),
  Retry: vi.fn(),
}));

const backend = vi.mocked(Backend);

function resetStore() {
  useAppStore.setState({
    configInfo: null,
    stations: [],
    stationNames: {},
    configFile: 'config.yml',
    activityLogs: [],
    loading: true,
    status: null,
    downloads: null,
  });
}

type Status = Awaited<ReturnType<typeof Backend.GetStatus>>;

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
    backend.GetStationNames.mockResolvedValue({ TBS: 'Tokyo Broadcasting System', FMT: 'Tokyo FM' });
    await useAppStore.getState().loadInitialData();

    expect(backend.GetConfig).toHaveBeenCalledOnce();
    expect(backend.GetAvailableStations).toHaveBeenCalledOnce();
    expect(useAppStore.getState()).toMatchObject({
      configInfo: cfg,
      stations: ['TBS', 'FMT'],
      stationNames: { TBS: 'Tokyo Broadcasting System', FMT: 'Tokyo FM' },
      loading: false,
    });
  });

  it('normalizes a null station response to an empty list', async () => {
    backend.GetAvailableStations.mockResolvedValue(null as unknown as string[]);
    backend.GetStationNames.mockResolvedValue({});

    await useAppStore.getState().loadStations();

    expect(useAppStore.getState().stations).toEqual([]);
    expect(useAppStore.getState().stationLabel('UNKNOWN')).toBe('UNKNOWN');
  });

  it('refreshes configuration and stations after loading a file', async () => {
    const cfg = { AreaID: 'JP27' } as Awaited<ReturnType<typeof Backend.GetConfig>>;
    backend.LoadConfig.mockResolvedValue(undefined);
    backend.GetConfig.mockResolvedValue(cfg);
    backend.GetAvailableStations.mockResolvedValue(['FM802']);
    backend.GetStationNames.mockResolvedValue({ FM802: 'FM802' });

    await useAppStore.getState().loadConfig('/tmp/radikron.yml');

    expect(backend.LoadConfig).toHaveBeenCalledWith('/tmp/radikron.yml');
    expect(useAppStore.getState()).toMatchObject({ configInfo: cfg, stations: ['FM802'], stationNames: { FM802: 'FM802' } });
  });

  it('retries station catalog refresh and updates visible stations', async () => {
    backend.RefreshStations.mockResolvedValue(['TBS', 'QRR']);
    backend.GetStationNames.mockResolvedValue({ TBS: 'Tokyo Broadcasting System', QRR: '文化放送' });

    await useAppStore.getState().refreshStations();

    expect(backend.RefreshStations).toHaveBeenCalledOnce();
    expect(useAppStore.getState().stations).toEqual(['TBS', 'QRR']);
    expect(useAppStore.getState().stationLabel('QRR')).toBe('文化放送');
  });

  it('retries failures and reloads the data that recovered', async () => {
    const failed = { configError: 'yaml: bad indentation' } as Status;
    const recovered = { monitoring: true, configError: '' } as Status;
    backend.Retry.mockResolvedValue(undefined);
    backend.GetConfig.mockResolvedValue({ AreaID: 'JP13' } as Awaited<ReturnType<typeof Backend.GetConfig>>);
    backend.GetAvailableStations.mockResolvedValue(['TBS']);
    backend.GetStationNames.mockResolvedValue({});
    backend.GetStatus.mockResolvedValue(recovered);
    useAppStore.setState({ status: failed });

    await useAppStore.getState().retry();

    expect(backend.Retry).toHaveBeenCalledOnce();
    expect(useAppStore.getState().status).toEqual(recovered);
    expect(useAppStore.getState().configInfo?.AreaID).toBe('JP13');
    expect(useAppStore.getState().stations).toEqual(['TBS']);
  });

  it('logs a failed retry and keeps the reported status', async () => {
    const stillFailing = { stationsError: 'radiko is unreachable' } as Status;
    backend.Retry.mockRejectedValue(new Error('failed to fetch station catalog'));
    backend.GetConfig.mockResolvedValue({ AreaID: 'JP13' } as Awaited<ReturnType<typeof Backend.GetConfig>>);
    backend.GetAvailableStations.mockResolvedValue([]);
    backend.GetStationNames.mockResolvedValue({});
    backend.GetStatus.mockResolvedValue(stillFailing);

    await useAppStore.getState().retry();

    const logs = useAppStore.getState().activityLogs;
    expect(logs[logs.length - 1]?.message).toBe('Retry failed: failed to fetch station catalog');
    expect(useAppStore.getState().status).toEqual(stillFailing);
  });

  it('loads downloads and keeps the last list when loading fails', async () => {
    const downloads = { scheduled: [], recent: [] } as unknown as Awaited<ReturnType<typeof Backend.GetDownloads>>;
    backend.GetDownloads.mockResolvedValueOnce(downloads);
    await useAppStore.getState().loadDownloads();
    expect(useAppStore.getState().downloads).toBe(downloads);

    backend.GetDownloads.mockRejectedValueOnce(new Error('asset not initialized'));
    await useAppStore.getState().loadDownloads();
    expect(useAppStore.getState().downloads).toBe(downloads);
  });
});
