import React, { useEffect } from 'react';
import { useThemeStore } from '@/store/useThemeStore';

interface ThemeProviderProps {
  children: React.ReactNode;
}

export const ThemeProvider: React.FC<ThemeProviderProps> = ({ children }) => {
  const theme = useThemeStore((state) => state.theme);
  const applyThemeToDocument = useThemeStore((state) => state.applyThemeToDocument);

  // Apply theme on mount and when it changes
  useEffect(() => {
    applyThemeToDocument();
  }, [theme, applyThemeToDocument]);

  // Listen for system theme changes when theme is set to 'system'
  useEffect(() => {
    if (theme === 'system') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      const handleChange = () => {
        applyThemeToDocument();
      };
      mediaQuery.addEventListener('change', handleChange);
      return () => mediaQuery.removeEventListener('change', handleChange);
    }
  }, [theme, applyThemeToDocument]);

  return <>{children}</>;
};

