import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import * as config from '../../../bindings/github.com/iomz/radikron/internal/config/models';
import { useAppStore } from '@/store/useAppStore';
import { Rules } from './Rules';

vi.mock('../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  AddRule: vi.fn(),
  ChooseRuleFolder: vi.fn(),
  GetConfig: vi.fn(),
  PreviewRule: vi.fn(),
  RemoveRule: vi.fn(),
  UpdateRule: vi.fn(),
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const backend = vi.mocked(Backend);

const cfg = config.Config.createFrom({
  DownloadDir: '/music/radiko',
  Rules: [{ Name: 'Midday', Title: 'MIDDAY LOUNGE', StationID: 'FMJ', DoW: ['sun'], Folder: 'midday', Exclude: { Pfm: 'GUEST' } }],
});

describe('Rules', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    backend.GetConfig.mockResolvedValue(cfg);
    backend.PreviewRule.mockResolvedValue([]);
    useAppStore.setState({ configInfo: cfg, stations: ['FMJ'], stationNames: { FMJ: 'J-WAVE' }, downloads: null, activityLogs: [] });
  });

  afterEach(() => cleanup());

  it('summarizes each rule', () => {
    render(<Rules onNavigate={vi.fn()} />);
    expect(screen.getByText('Title has “MIDDAY LOUNGE” · on J-WAVE · Sun · skips “GUEST”')).toBeTruthy();
    expect(screen.getByText('/music/radiko/midday')).toBeTruthy();
    expect(screen.getByText('Nothing scheduled')).toBeTruthy();
  });

  it('saves an edited rule under its original name and keeps its skips', async () => {
    backend.UpdateRule.mockResolvedValue(undefined);
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Midday/ }));
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Lunch' } });
    fireEvent.click(screen.getByRole('button', { name: 'Mon' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(backend.UpdateRule).toHaveBeenCalled());
    const [name, rule] = backend.UpdateRule.mock.calls[0];
    expect(name).toBe('Midday');
    expect(rule?.Name).toBe('Lunch');
    expect(rule?.DoW).toEqual(['sun', 'mon']);
    expect(rule?.Exclude?.Pfm).toBe('GUEST');
  });

  it('explains what a new rule is missing before it can be created', () => {
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'New rule' }));
    const create = screen.getByRole('button', { name: 'Create rule' }) as HTMLButtonElement;
    expect((screen.getByLabelText('Name') as HTMLInputElement).value).toMatch(/^rule-\d{14}$/);
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: '' } });
    expect(create.disabled).toBe(true);
    expect(screen.getByText('Give the rule a name.')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Midday' } });
    expect(screen.getByText('A rule named “Midday” already exists.')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Jazz' } });
    expect(screen.getByText('Fill in a title, host, or keyword to match.')).toBeTruthy();
    fireEvent.change(screen.getAllByLabelText('Keyword')[0], { target: { value: 'jazz' } });
    expect(create.disabled).toBe(false);
  });

  it('keeps Folder and Skip programs closed, with their values in the header', () => {
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Midday/ }));
    const folder = screen.getByText('Folder').closest('details')!;
    const skip = screen.getByText('Skip programs').closest('details')!;
    expect(folder.open).toBe(false);
    expect(skip.open).toBe(false);
    expect(skip.querySelector('summary')?.textContent).toContain('“GUEST”');
  });

  it('previews the programs a rule matches', async () => {
    backend.PreviewRule.mockResolvedValue([
      { Title: 'MIDDAY LOUNGE', StationID: 'FMJ', Ft: '29990101130000', To: '29990101140000' },
      { Title: 'Gone', StationID: 'FMJ', Ft: '20000101130000', To: '20000101140000' },
    ] as Awaited<ReturnType<typeof Backend.PreviewRule>>);
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Midday/ }));
    expect(await screen.findByText('1 program matches: 0 aired, 1 upcoming.', {}, { timeout: 2000 })).toBeTruthy();
    expect(screen.getByText('MIDDAY LOUNGE')).toBeTruthy();
    expect(screen.queryByText('Gone')).toBeNull();
    expect(backend.PreviewRule.mock.calls[0][0]?.Title).toBe('MIDDAY LOUNGE');
  });

  it('chooses the folder with a picker', async () => {
    backend.ChooseRuleFolder.mockResolvedValue('/Volumes/Radio/jazz');
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Midday/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Choose…' }));
    // A folder outside the download folder is used as is.
    expect(await screen.findAllByText('/Volumes/Radio/jazz')).toHaveLength(2);
    expect(backend.ChooseRuleFolder).toHaveBeenCalledWith('midday');
    fireEvent.click(screen.getByRole('button', { name: 'Use the download folder' }));
    expect(screen.getAllByText('/music/radiko').length).toBeGreaterThan(0);
  });

  it('deletes a rule after confirmation', async () => {
    backend.RemoveRule.mockResolvedValue(undefined);
    render(<Rules onNavigate={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Midday/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete rule' }));
    expect(backend.RemoveRule).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await waitFor(() => expect(backend.RemoveRule).toHaveBeenCalledWith('Midday'));
  });

  it('points to Search when there are no rules', () => {
    const onNavigate = vi.fn();
    useAppStore.setState({ configInfo: config.Config.createFrom({ Rules: [] }) });
    render(<Rules onNavigate={onNavigate} />);
    fireEvent.click(screen.getByRole('button', { name: 'Go to Search' }));
    expect(onNavigate).toHaveBeenCalledWith('search');
  });
});
