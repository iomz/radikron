import React from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Events } from '@wailsio/runtime';
import { toast } from 'sonner';
import * as Backend from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { UpdateButton } from './UpdateButton';

vi.mock('../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app', () => ({
  AvailableUpdate: vi.fn(),
  CheckForUpdates: vi.fn(),
  UpdatesSupported: vi.fn(),
}));

vi.mock('@wailsio/runtime', () => ({ Events: { On: vi.fn() } }));
vi.mock('sonner', () => ({ toast: vi.fn() }));

const backend = vi.mocked(Backend);
const eventsOn = vi.mocked(Events.On);
let updateAvailable: ((version: string) => void) | undefined;

describe('UpdateButton', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    updateAvailable = undefined;
    eventsOn.mockImplementation((name, callback) => {
      if (name === 'update-available') {
        const handler = callback as (event: { data: { version: string } }) => void;
        updateAvailable = (version) => handler({ data: { version } });
      }
      return vi.fn();
    });
    backend.CheckForUpdates.mockResolvedValue(undefined);
    backend.AvailableUpdate.mockResolvedValue('');
  });

  afterEach(() => cleanup());

  it('renders nothing when the build cannot update itself', async () => {
    backend.UpdatesSupported.mockResolvedValue(false);
    const { container } = render(<UpdateButton />);
    await waitFor(() => expect(backend.UpdatesSupported).toHaveBeenCalled());
    expect(container.innerHTML).toBe('');
    expect(backend.AvailableUpdate).not.toHaveBeenCalled();
  });

  it('opens the update window when clicked', async () => {
    backend.UpdatesSupported.mockResolvedValue(true);
    render(<UpdateButton />);
    fireEvent.click(await screen.findByRole('button', { name: 'Check for updates' }));
    expect(backend.CheckForUpdates).toHaveBeenCalledTimes(1);
  });

  it('announces updates found before and after mounting', async () => {
    backend.UpdatesSupported.mockResolvedValue(true);
    backend.AvailableUpdate.mockResolvedValue('9.0.0');
    render(<UpdateButton />);
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith('Radikron 9.0.0 is available', expect.objectContaining({ id: 'radikron-update' }))
    );

    updateAvailable?.('9.1.0');
    expect(toast).toHaveBeenLastCalledWith('Radikron 9.1.0 is available', expect.objectContaining({ id: 'radikron-update' }));

    const options = vi.mocked(toast).mock.lastCall?.[1] as unknown as { action: { onClick: () => void } };
    options.action.onClick();
    expect(backend.CheckForUpdates).toHaveBeenCalledTimes(1);
  });
});
