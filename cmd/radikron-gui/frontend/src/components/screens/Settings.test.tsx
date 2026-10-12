import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Browser } from '@wailsio/runtime';
import * as Backend from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from '@/store/useAppStore';
import { Settings } from './Settings';

vi.mock('../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  ChooseDownloadFolder: vi.fn(),
  ExportConfigFile: vi.fn(),
  GetAllStations: vi.fn(),
  GetConfigFile: vi.fn(),
  GetSettings: vi.fn(),
  ImportConfigFile: vi.fn(),
  OpenDirectory: vi.fn(),
  SaveSettings: vi.fn(),
}));
vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wailsio/runtime')>()),
  Browser: { OpenURL: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const backend = vi.mocked(Backend);
type Saved = Awaited<ReturnType<typeof Backend.GetSettings>>;

const current = {
  areaId: 'JP13',
  downloadDir: '/music/radiko',
  fileFormat: 'aac',
  minimumOutputSizeMb: 1,
  extraStations: [],
  ignoreStations: [],
  maxDownloadingConcurrency: 64,
  maxEncodingConcurrency: 2,
} as Saved;

describe('Settings', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    backend.GetSettings.mockResolvedValue(current);
    backend.GetAllStations.mockResolvedValue(['TBS', 'QRR', 'ABC']);
    backend.GetConfigFile.mockResolvedValue('/config/config.yml');
    backend.SaveSettings.mockResolvedValue(undefined);
    useAppStore.setState({
      stations: ['TBS', 'QRR'],
      stationNames: { TBS: 'TBSラジオ', QRR: '文化放送', ABC: 'ABCラジオ' },
      activityLogs: [{ id: 1, type: 'error', message: 'Download failed', timestamp: '2026-10-12T05:21:00Z' }],
      loadConfigInfo: vi.fn().mockResolvedValue(undefined),
      loadStations: vi.fn().mockResolvedValue(undefined),
    });
  });

  afterEach(() => cleanup());

  it('shows the current settings and the activity log', async () => {
    render(<Settings />);
    expect(await screen.findByText('/music/radiko')).toBeTruthy();
    expect(screen.getByText('/config/config.yml')).toBeTruthy();
    expect(screen.getByRole('radio', { name: 'AAC' }).getAttribute('aria-checked')).toBe('true');
    expect((screen.getByLabelText('Area') as HTMLSelectElement).value).toBe('JP13');
    expect(screen.getByText('Watching 2 stations')).toBeTruthy();
    expect(screen.getByText('Download failed')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Save' })).toBeNull();
  });

  it('saves changed settings together', async () => {
    render(<Settings />);
    fireEvent.click(await screen.findByRole('radio', { name: 'MP3' }));
    fireEvent.change(screen.getByLabelText('Also watch'), { target: { value: 'ABC' } });
    fireEvent.change(screen.getByLabelText('Ignore'), { target: { value: 'QRR' } });
    expect(screen.getByText('Save to update this list.')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() =>
      expect(backend.SaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ fileFormat: 'mp3', extraStations: ['ABC'], ignoreStations: ['QRR'], areaId: 'JP13' }),
      ),
    );
  });

  it('discards changes and removes a chosen station', async () => {
    backend.GetSettings.mockResolvedValue({ ...current, extraStations: ['ABC'] } as Saved);
    render(<Settings />);
    fireEvent.click(await screen.findByRole('button', { name: 'Remove ABCラジオ' }));
    expect(screen.getByText('No stations from other areas.')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));
    expect(screen.getByRole('button', { name: 'Remove ABCラジオ' })).toBeTruthy();
  });

  it('shows why a save failed and keeps the changes', async () => {
    backend.SaveSettings.mockRejectedValue(new Error('failed to save config: read-only'));
    render(<Settings />);
    fireEvent.click(await screen.findByRole('radio', { name: 'MP3' }));
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect((await screen.findByRole('alert')).textContent).toContain('read-only');
    expect(screen.getByRole('radio', { name: 'MP3' }).getAttribute('aria-checked')).toBe('true');
  });

  it('uses the folder the user picks', async () => {
    backend.ChooseDownloadFolder.mockResolvedValue('/elsewhere');
    render(<Settings />);
    fireEvent.click(await screen.findByRole('button', { name: 'Choose…' }));
    expect(await screen.findByText('/elsewhere')).toBeTruthy();
    expect(backend.ChooseDownloadFolder).toHaveBeenCalledWith('/music/radiko');
  });

  it('opens a watched station on radiko', async () => {
    render(<Settings />);
    fireEvent.click(await screen.findByRole('link', { name: '文化放送' }));
    expect(Browser.OpenURL).toHaveBeenCalledWith('https://radiko.jp/#!/live/QRR');
  });

  it('imports a config file after the user confirms', async () => {
    backend.ImportConfigFile.mockResolvedValue('/elsewhere/config.yml');
    render(<Settings />);
    fireEvent.click(await screen.findByRole('button', { name: 'Import config…' }));
    expect(screen.getByText(/replaces your settings and rules/)).toBeTruthy();
    expect(backend.ImportConfigFile).not.toHaveBeenCalled();
    backend.GetSettings.mockResolvedValue({ ...current, fileFormat: 'mp3' } as Saved);
    fireEvent.click(screen.getByRole('button', { name: 'Choose a file…' }));
    await waitFor(() => expect(screen.getByRole('radio', { name: 'MP3' }).getAttribute('aria-checked')).toBe('true'));
    expect(useAppStore.getState().loadConfigInfo).toHaveBeenCalled();
    expect(screen.queryByText(/replaces your settings and rules/)).toBeNull();
  });
});
