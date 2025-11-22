import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

type Theme = 'light' | 'dark' | 'system';

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  getEffectiveTheme: () => 'light' | 'dark';
  applyThemeToDocument: () => void;
  setupSystemThemeListener: () => (() => void) | undefined;
  cleanupSystemThemeListener: (() => void) | undefined;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'light', // Default to light theme
      cleanupSystemThemeListener: undefined,
      setTheme: (theme) => {
        const currentCleanup = get().cleanupSystemThemeListener;
        
        // Clean up existing listener if any
        if (currentCleanup) {
          currentCleanup();
          set({ cleanupSystemThemeListener: undefined });
        }
        
        set({ theme });
        // Apply theme immediately when changed
        if (typeof window !== 'undefined') {
          get().applyThemeToDocument();
          
          // Set up listener if theme is 'system'
          if (theme === 'system') {
            const cleanup = get().setupSystemThemeListener();
            if (cleanup) {
              set({ cleanupSystemThemeListener: cleanup });
            }
          }
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
      setupSystemThemeListener: () => {
        if (typeof window === 'undefined') return undefined;
        
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        const handleChange = () => {
          // Only react if theme is still 'system'
          if (get().theme === 'system') {
            get().applyThemeToDocument();
          }
        };
        
        // Add listener (use addEventListener for modern browsers)
        if (mediaQuery.addEventListener) {
          mediaQuery.addEventListener('change', handleChange);
          return () => {
            mediaQuery.removeEventListener('change', handleChange);
          };
        } else {
          // Fallback for older browsers (deprecated but still supported)
          mediaQuery.addListener(handleChange);
          return () => {
            mediaQuery.removeListener(handleChange);
          };
        }
      },
    }),
    {
      name: 'radikron-theme',
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        theme: state.theme,
        // Exclude cleanupSystemThemeListener and methods from persistence
      }),
      onRehydrateStorage: () => (state) => {
        // Apply theme after rehydration
        if (state && typeof window !== 'undefined') {
          state.applyThemeToDocument();
          
          // Set up system theme listener if theme is 'system'
          if (state.theme === 'system') {
            const cleanup = state.setupSystemThemeListener();
            if (cleanup) {
              state.cleanupSystemThemeListener = cleanup;
            }
          }
        }
      },
    }
  )
);

