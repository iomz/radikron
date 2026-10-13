import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from '@/store/useAppStore';
import { todayBroadcastDay } from '@/lib/timetable';
import { Timetable } from './Timetable';

vi.mock('../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({ GetSchedules: vi.fn() }));
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }));

const backend = vi.mocked(Backend);
type DownloadsData = Awaited<ReturnType<typeof Backend.GetDownloads>>;

describe('Timetable', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    backend.GetSchedules.mockResolvedValue([]);
    const day = todayBroadcastDay(new Date());
    useAppStore.setState({
      stationNames: { TBS: 'TBSラジオ' },
      downloads: {
        scheduled: [
          {
            id: 'p', stationId: 'TBS', title: 'Late Show', start: `${day}200000`, end: `${day}210000`, ruleName: 'Talk',
            manual: false, state: 'waiting', stage: '', completed: 0, total: 0, error: '', updatedAt: '',
          },
        ],
        recent: [],
      } as unknown as DownloadsData,
    });
  });

  afterEach(() => cleanup());

  it('shows a summary on hover and details on click', async () => {
    render(<Timetable />);
    const block = screen.getByRole('button', { name: /Late Show/ });
    fireEvent.mouseEnter(block);
    const card = screen.getByRole('tooltip');
    expect(card.textContent).toContain('20:00–21:00');
    expect(card.textContent).toContain('Rule · Talk');
    fireEvent.mouseLeave(block);
    expect(screen.queryByRole('tooltip')).toBeNull();

    fireEvent.click(block);
    expect(await screen.findByRole('dialog')).toBeTruthy();
  });
});
