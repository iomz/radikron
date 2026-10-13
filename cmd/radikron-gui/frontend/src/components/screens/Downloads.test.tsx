import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from '@/store/useAppStore';
import { Downloads } from './Downloads';

vi.mock('../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  ClearHistory: vi.fn(),
  GetDownloads: vi.fn(),
  GetSchedules: vi.fn(),
  SkipDownload: vi.fn(),
  UndoSkip: vi.fn(),
}));
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const backend = vi.mocked(Backend);
type DownloadsData = Awaited<ReturnType<typeof Backend.GetDownloads>>;
type Schedules = Awaited<ReturnType<typeof Backend.GetSchedules>>;

const row = (fields: Record<string, unknown>) => ({
  id: '', stationId: 'TBS', title: '', start: '29990101100000', end: '29990101110000', ruleName: '', manual: false,
  state: 'waiting', stage: '', completed: 0, total: 0, error: '', updatedAt: '', ...fields,
});

describe('Downloads', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    useAppStore.setState({
      stationNames: { TBS: 'TBSラジオ' },
      activityLogs: [],
      downloads: {
        scheduled: [
          row({ id: 'w', title: 'Waiting Show', ruleName: 'Jazz' }),
          row({ id: 'm', title: 'Manual Show', manual: true, start: '29990102100000' }),
          row({ id: 'f', title: 'Failed Show', state: 'failed', stage: 'encoding', error: 'ffmpeg not found', start: '29990103100000' }),
        ],
        recent: [row({ id: 'd', title: 'Downloaded Show', state: 'completed', start: '20261011100000' })],
      } as unknown as DownloadsData,
    });
  });

  afterEach(() => cleanup());

  it('filters by state', () => {
    render(<Downloads />);
    expect(screen.getByText('Waiting Show')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Failed 1' }));
    expect(screen.queryByText('Waiting Show')).toBeNull();
    expect(screen.getByText('encoding: ffmpeg not found')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Completed 1' }));
    expect(screen.getByText('Downloaded Show')).toBeTruthy();
    expect(screen.queryByText('Failed Show')).toBeNull();
  });

  it('shows program details', async () => {
    backend.GetSchedules.mockResolvedValue([
      { ID: 'w', StationID: 'TBS', Title: 'Waiting Show', Ft: '29990101100000', To: '29990101110000', Pfm: 'Host Name' },
    ] as unknown as Schedules);
    render(<Downloads />);
    fireEvent.click(screen.getByRole('button', { name: 'Waiting Show' }));
    expect(await screen.findByText('TBSラジオ · Host Name')).toBeTruthy();
  });

  it('skips a program, keeps it out of All, and undoes the skip', async () => {
    backend.SkipDownload.mockResolvedValue(undefined);
    backend.UndoSkip.mockResolvedValue(undefined);
    backend.GetDownloads.mockResolvedValue({
      scheduled: [],
      recent: [],
      skipped: [row({ id: 'w', title: 'Waiting Show', state: 'skipped' })],
    } as unknown as DownloadsData);
    render(<Downloads />);
    fireEvent.click(screen.getByRole('button', { name: 'Skip Waiting Show' }));
    await waitFor(() => expect(backend.SkipDownload).toHaveBeenCalledWith('TBS', '29990101100000'));

    await waitFor(() => expect(screen.queryByText('Waiting Show')).toBeNull());
    fireEvent.click(screen.getByRole('button', { name: 'Skipped 1' }));
    fireEvent.click(screen.getByRole('button', { name: 'Undo skip Waiting Show' }));
    await waitFor(() => expect(backend.UndoSkip).toHaveBeenCalledWith('TBS', '29990101100000'));
  });

  it('cancels a download in progress', async () => {
    backend.SkipDownload.mockResolvedValue(undefined);
    backend.GetDownloads.mockResolvedValue({ scheduled: [], recent: [], skipped: [] } as unknown as DownloadsData);
    useAppStore.setState({
      downloads: { scheduled: [row({ title: 'Live Show', state: 'downloading' })], recent: [], skipped: [] } as unknown as DownloadsData,
    });
    render(<Downloads />);
    expect(screen.queryByRole('button', { name: 'Skip Live Show' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel Live Show' }));
    await waitFor(() => expect(backend.SkipDownload).toHaveBeenCalledWith('TBS', '29990101100000'));
  });

  it('clears the history after the user confirms', async () => {
    backend.ClearHistory.mockResolvedValue(undefined);
    backend.GetDownloads.mockResolvedValue({ scheduled: [], recent: [], skipped: [] } as unknown as DownloadsData);
    render(<Downloads />);
    fireEvent.click(screen.getByRole('button', { name: 'Clear history' }));
    const dialog = screen.getByRole('dialog', { name: 'Clear the download history?' });
    expect(backend.ClearHistory).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Clear history' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(backend.ClearHistory).toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Clear history' }).hasAttribute('disabled')).toBe(true);
  });
});
