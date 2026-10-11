import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import * as Backend from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { ProgramSearchBrowser } from './ProgramSearchBrowser';

vi.mock('../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  FetchProgramSnapshots: vi.fn(),
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
});
