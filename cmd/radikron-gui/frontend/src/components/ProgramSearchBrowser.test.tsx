import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { ProgramSearchBrowser } from './ProgramSearchBrowser';

vi.mock('../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  AddRule: vi.fn(),
  FetchProgramSnapshots: vi.fn(),
  GetConfig: vi.fn(),
  RuleFromProgram: vi.fn(),
  SearchWeeklyPrograms: vi.fn(),
}));

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wailsio/runtime')>()),
  Browser: { OpenURL: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const backend = vi.mocked(Backend);

describe('ProgramSearchBrowser', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    backend.FetchProgramSnapshots.mockResolvedValue(undefined);
    backend.SearchWeeklyPrograms.mockResolvedValue([]);
  });

  afterEach(() => cleanup());

  it('searches only when the form is submitted', async () => {
    render(<ProgramSearchBrowser />);
    const keyword = screen.getByLabelText('Keyword');
    await waitFor(() =>
      expect((screen.getByRole('button', { name: 'Search' }) as HTMLButtonElement).disabled).toBe(false)
    );

    fireEvent.change(keyword, { target: { value: 'シティポップ' } });
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(backend.SearchWeeklyPrograms).not.toHaveBeenCalled();
    screen.getByText('Enter a keyword above to search for programs.');

    fireEvent.submit(keyword.closest('form')!);
    await waitFor(() =>
      expect(backend.SearchWeeklyPrograms).toHaveBeenCalledWith('', '', 'シティポップ', '')
    );
    await screen.findByText(/No programs found/);
  });

  it('accepts a new search while one is pending and keeps the newest results', async () => {
    type SearchResult = Awaited<ReturnType<typeof Backend.SearchWeeklyPrograms>>;
    let resolveFirst: (value: SearchResult) => void = () => undefined;
    backend.SearchWeeklyPrograms.mockImplementationOnce(
      () =>
        new Promise<SearchResult>((resolve) => {
          resolveFirst = resolve;
        }) as ReturnType<typeof Backend.SearchWeeklyPrograms>
    );
    backend.SearchWeeklyPrograms.mockResolvedValueOnce([
      { ID: 'new', Title: 'New result', StationID: 'TBS', Ft: '20260101000000', To: '20260101010000' },
    ] as SearchResult);

    render(<ProgramSearchBrowser />);
    const keyword = screen.getByLabelText('Keyword');
    const form = keyword.closest('form')!;
    await waitFor(() =>
      expect((screen.getByRole('button', { name: 'Search' }) as HTMLButtonElement).disabled).toBe(false)
    );

    fireEvent.change(keyword, { target: { value: 'old' } });
    fireEvent.submit(form);
    await screen.findByRole('button', { name: 'Searching...' });
    expect((screen.getByRole('button', { name: 'Searching...' }) as HTMLButtonElement).disabled).toBe(false);

    fireEvent.change(keyword, { target: { value: 'new' } });
    fireEvent.submit(form);
    await screen.findByText('New result');

    resolveFirst([{ ID: 'old', Title: 'Old result', StationID: 'TBS' }] as SearchResult);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText('Old result')).toBeNull();
    expect(backend.SearchWeeklyPrograms).toHaveBeenLastCalledWith('', '', 'new', '');
  });

  it('offers the action that fits the air time and creates a rule from a result', async () => {
    type SearchResult = Awaited<ReturnType<typeof Backend.SearchWeeklyPrograms>>;
    type Rule = Awaited<ReturnType<typeof Backend.RuleFromProgram>>;
    backend.SearchWeeklyPrograms.mockResolvedValue([
      { ID: 'past', Title: 'Past Show', StationID: 'TBS', Ft: '20000101100000', To: '20000101110000' },
      { ID: 'future', Title: 'Future Show', StationID: 'TBS', Ft: '29990101100000', To: '29990101110000' },
    ] as SearchResult);
    backend.RuleFromProgram.mockResolvedValue({ Name: 'Future Show', Title: 'Future Show', StationID: 'TBS' } as Rule);
    backend.AddRule.mockResolvedValue(undefined);
    backend.GetConfig.mockResolvedValue(null);

    render(<ProgramSearchBrowser />);
    const keyword = screen.getByLabelText('Keyword');
    await waitFor(() =>
      expect((screen.getByRole('button', { name: 'Search' }) as HTMLButtonElement).disabled).toBe(false)
    );
    fireEvent.change(keyword, { target: { value: 'Show' } });
    fireEvent.submit(keyword.closest('form')!);

    fireEvent.click(await screen.findByText('Past Show'));
    const expired = await screen.findByRole('button', { name: 'No longer available' });
    expect((expired as HTMLButtonElement).disabled).toBe(true);
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('button', { name: 'No longer available' })).toBeNull());

    fireEvent.click(await screen.findByText('Future Show'));
    await screen.findByRole('button', { name: 'Schedule download' });
    fireEvent.click(screen.getByRole('button', { name: 'Create rule…' }));
    const name = await screen.findByLabelText('Rule name');
    fireEvent.change(name, { target: { value: 'My show' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create rule' }));

    await waitFor(() =>
      expect(backend.AddRule).toHaveBeenCalledWith(expect.objectContaining({ Name: 'My show', Title: 'Future Show' }))
    );
  });
});
