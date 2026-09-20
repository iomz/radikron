import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useThemeStore } from './useThemeStore';

describe('useThemeStore', () => {
  beforeEach(() => {
    useThemeStore.getState().cleanupSystemThemeListener?.();
    useThemeStore.setState({ theme: 'light', cleanupSystemThemeListener: undefined });
    document.documentElement.classList.remove('dark');
    localStorage.clear();
  });

  it('applies explicit themes to document', () => {
    useThemeStore.getState().setTheme('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);

    useThemeStore.getState().setTheme('light');
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('tracks system theme and cleans listener when mode changes', () => {
    const addEventListener = vi.fn();
    const removeEventListener = vi.fn();
    window.matchMedia = vi.fn().mockReturnValue({
      matches: true,
      addEventListener,
      removeEventListener,
    });

    useThemeStore.getState().setTheme('system');

    expect(document.documentElement.classList.contains('dark')).toBe(true);
    expect(addEventListener).toHaveBeenCalledWith('change', expect.any(Function));

    useThemeStore.getState().setTheme('light');
    expect(removeEventListener).toHaveBeenCalledWith('change', expect.any(Function));
    expect(useThemeStore.getState().cleanupSystemThemeListener).toBeUndefined();
  });
});
