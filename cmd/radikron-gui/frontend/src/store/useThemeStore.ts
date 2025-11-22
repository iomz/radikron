import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

type Theme = 'light' | 'dark' | 'system';

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  getEffectiveTheme: () => 'light' | 'dark';
  applyThemeToDocument: () => void;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'light', // Default to light theme
      setTheme: (theme) => {
        set({ theme });
        // Apply theme immediately when changed
        if (typeof window !== 'undefined') {
          get().applyThemeToDocument();
        }
      },
      getEffectiveTheme: () => {
        const { theme } = get();
        if (theme === 'system') {
          if (typeof window === 'undefined') return 'light';
          return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        }
        return theme;
      },
      applyThemeToDocument: () => {
        if (typeof window === 'undefined') return;
        
        const root = document.documentElement;
        const effectiveTheme = get().getEffectiveTheme();
        
        // Remove dark class for light theme, add it for dark theme
        if (effectiveTheme === 'dark') {
          root.classList.add('dark');
        } else {
          root.classList.remove('dark');
        }
      },
    }),
    {
      name: 'radikron-theme',
      storage: createJSONStorage(() => localStorage),
      onRehydrateStorage: () => (state) => {
        // Apply theme after rehydration
        if (state && typeof window !== 'undefined') {
          state.applyThemeToDocument();
        }
      },
    }
  )
);

