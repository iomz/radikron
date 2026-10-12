import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from '@/store/useAppStore';
import { Downloads } from './Downloads';

vi.mock('../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  DeleteManualInjection: vi.fn(),
  GetDownloads: vi.fn(),
  GetSchedules: vi.fn(),
}));
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

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
    fireEvent.click(screen.getByRole('button', { name: 'Downloaded 1' }));
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

  it('removes a manual program after confirmation', async () => {
    backend.DeleteManualInjection.mockResolvedValue(undefined);
    backend.GetDownloads.mockResolvedValue({ scheduled: [], recent: [] } as unknown as DownloadsData);
    render(<Downloads />);
    expect(screen.getAllByRole('button', { name: 'Remove' })).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(Array.from(dialog.querySelectorAll('button')).find((b) => b.textContent === 'Remove')!);
    await waitFor(() => expect(backend.DeleteManualInjection).toHaveBeenCalledWith('m'));
  });
});
