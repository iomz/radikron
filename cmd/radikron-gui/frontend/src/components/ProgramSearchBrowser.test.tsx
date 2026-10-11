import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
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
});
