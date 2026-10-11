import React from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useThemeStore } from '@/store/useThemeStore';
import { ThemeToggle } from './ThemeToggle';

describe('ThemeToggle', () => {
  beforeEach(() => {
    window.matchMedia = vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    });
    useThemeStore.setState({ theme: 'system', cleanupSystemThemeListener: undefined });
  });

  afterEach(() => cleanup());

  it('marks the current theme and switches on click', () => {
    render(<ThemeToggle />);
    const pressed = (name: string) => screen.getByRole('button', { name }).getAttribute('aria-pressed');
    expect(pressed('System')).toBe('true');

    fireEvent.click(screen.getByRole('button', { name: 'Dark' }));
    expect(useThemeStore.getState().theme).toBe('dark');
    expect(pressed('Dark')).toBe('true');
    expect(pressed('System')).toBe('false');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
