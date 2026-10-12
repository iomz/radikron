import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { useAppStore } from '@/store/useAppStore';
import { Search } from './Search';
import { todayBroadcastDay } from '@/lib/timetable';

vi.mock('../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  AddRule: vi.fn(),
  FetchProgramSnapshots: vi.fn(),
  GetConfig: vi.fn(),
  GetDesignatedFolder: vi.fn(),
  GetDownloads: vi.fn(),
  InjectProgram: vi.fn(),
  RuleFromProgram: vi.fn(),
  SearchWeeklyPrograms: vi.fn(),
}));
vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wailsio/runtime')>()),
  Browser: { OpenURL: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const backend = vi.mocked(Backend);
type SearchResult = Awaited<ReturnType<typeof Backend.SearchWeeklyPrograms>>;
type Rule = Awaited<ReturnType<typeof Backend.RuleFromProgram>>;

const results = [
  { ID: 'old', Title: 'Old Show', StationID: 'TBS', Ft: '20000101100000', To: '20000101110000' },
  { ID: 'future', Title: 'Future Show', StationID: 'TBS', Ft: '29990101100000', To: '29990101110000', Desc: 'About it' },
] as SearchResult;

async function searchFor(text: string) {
  render(<Search />);
  await screen.findByText(/Enter words to search/);
  const keyword = screen.getByLabelText('Keyword');
  fireEvent.change(keyword, { target: { value: text } });
}

describe('Search', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    backend.FetchProgramSnapshots.mockResolvedValue(undefined);
    backend.SearchWeeklyPrograms.mockResolvedValue([]);
    backend.GetDesignatedFolder.mockResolvedValue('/music/radiko');
    backend.GetDownloads.mockResolvedValue({ scheduled: [], recent: [] } as unknown as Awaited<ReturnType<typeof Backend.GetDownloads>>);
    useAppStore.setState({ stations: ['TBS'], stationNames: { TBS: 'TBSラジオ' }, configInfo: null, downloads: null, activityLogs: [] });
  });

  afterEach(() => cleanup());

  it('searches once typing pauses', async () => {
    render(<Search />);
    await screen.findByText(/Enter words to search/);
    fireEvent.change(screen.getByLabelText('Keyword'), { target: { value: 'シティ' } });
    fireEvent.change(screen.getByLabelText('Keyword'), { target: { value: 'シティポップ' } });
    expect(backend.SearchWeeklyPrograms).not.toHaveBeenCalled();
    await waitFor(() => expect(backend.SearchWeeklyPrograms).toHaveBeenCalledWith('', '', 'シティポップ', ''));
    expect(backend.SearchWeeklyPrograms).toHaveBeenCalledTimes(1);
    await screen.findByText(/No programs found/);
  });

  it('clears the search with the button inside the field', async () => {
    await searchFor('MIDDAY LOUNGE');
    fireEvent.click(screen.getByRole('button', { name: 'Clear search' }));
    expect((screen.getByLabelText('Keyword') as HTMLInputElement).value).toBe('');
    expect(screen.getByText(/Enter words to search/)).toBeTruthy();
  });

  it('offers actions by air time and schedules a program', async () => {
    backend.SearchWeeklyPrograms.mockResolvedValue(results);
    backend.InjectProgram.mockResolvedValue(undefined);
    await searchFor('Show');

    fireEvent.click(await screen.findByRole('button', { name: /Future Show/ }));
    // Programs radiko no longer has are not listed.
    expect(screen.queryByText('Old Show')).toBeNull();
    expect(screen.getByText('1 program for “Show”')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Schedule download' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Schedule' }));
    await waitFor(() => expect(backend.InjectProgram).toHaveBeenCalledWith(expect.objectContaining({ ID: 'future' }), ''));
  });

  it('creates a rule from a result', async () => {
    backend.SearchWeeklyPrograms.mockResolvedValue(results);
    backend.RuleFromProgram.mockResolvedValue({ Name: 'Future Show', Title: 'Future Show', StationID: 'TBS' } as Rule);
    backend.AddRule.mockResolvedValue(undefined);
    backend.GetConfig.mockResolvedValue(null);
    await searchFor('Show');

    fireEvent.click(await screen.findByRole('button', { name: /Future Show/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Create rule for every episode…' }));
    fireEvent.change(await screen.findByLabelText('Rule name'), { target: { value: 'My show' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create rule' }));
    await waitFor(() => expect(backend.AddRule).toHaveBeenCalledWith(expect.objectContaining({ Name: 'My show', Title: 'Future Show' })));
  });

  it('shows only the programs of the chosen day, even when radiko repeats a program ID', async () => {
    const today = todayBroadcastDay(new Date());
    // React leaves stale rows behind when rows share a key.
    const consoleError = vi.spyOn(console, 'error');
    backend.SearchWeeklyPrograms.mockResolvedValue([
      // radiko splits long programs into parts that share one ID.
      { ID: 'split', Title: 'Long Show (1)', StationID: 'TBS', Ft: '29990101200000', To: '29990101210000' },
      { ID: 'split', Title: 'Long Show (2)', StationID: 'TBS', Ft: '29990101210000', To: '29990101220000' },
      { ID: 'tonight', Title: 'Tonight Show', StationID: 'TBS', Ft: `${today}230000`, To: `${today}235900` },
    ] as SearchResult);
    await searchFor('Show');
    await screen.findByText('3 programs for “Show”');
    fireEvent.change(screen.getByLabelText('Day'), { target: { value: today } });
    await screen.findByText('1 program for “Show”');
    expect(screen.getAllByRole('article').map((a) => a.textContent)).toEqual([expect.stringContaining('Tonight Show')]);
    expect(consoleError).not.toHaveBeenCalledWith(expect.stringContaining('same key'), expect.anything(), expect.anything());
    consoleError.mockRestore();
  });
});
