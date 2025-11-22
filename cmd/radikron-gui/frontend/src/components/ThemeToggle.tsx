import React, { useState, useRef, useEffect } from 'react';
import { Moon, Sun, Monitor } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useThemeStore } from '@/store/useThemeStore';

export const ThemeToggle: React.FC = () => {
  const theme = useThemeStore((state) => state.theme);
  const setTheme = useThemeStore((state) => state.setTheme);
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };

    if (open) {
      document.addEventListener('mousedown', handleClickOutside);
      if (containerRef.current && menuRef.current) {
        const button = containerRef.current.querySelector('button');
        if (button) {
          const rect = button.getBoundingClientRect();
          menuRef.current.style.top = `${rect.bottom + 4}px`;
          menuRef.current.style.right = `${window.innerWidth - rect.right}px`;
        }
      }
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [open]);

  const handleThemeSelect = (newTheme: 'light' | 'dark' | 'system') => {
    setTheme(newTheme);
    setOpen(false);
  };

  return (
    <div ref={containerRef} className="relative">
      <Button
        variant="outline"
        size="icon"
        type="button"
        className="relative"
        onClick={() => setOpen(!open)}
        aria-label="Toggle theme"
        aria-expanded={open}
        aria-haspopup="true"
      >
        <Sun className="h-[1.2rem] w-[1.2rem] scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
        <Moon className="absolute h-[1.2rem] w-[1.2rem] scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
        <span className="sr-only">Toggle theme</span>
      </Button>
      {open && (
        <div
          ref={menuRef}
          className="z-[100] min-w-[8rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md"
          style={{ position: 'fixed' }}
          role="menu"
        >
          <div
            className="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors focus:bg-accent focus:text-accent-foreground hover:bg-accent hover:text-accent-foreground"
            onClick={() => handleThemeSelect('light')}
            role="menuitem"
          >
            <Sun className="mr-2 h-4 w-4" />
            <span>Light</span>
            {theme === 'light' && <span className="ml-auto">✓</span>}
          </div>
          <div
            className="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors focus:bg-accent focus:text-accent-foreground hover:bg-accent hover:text-accent-foreground"
            onClick={() => handleThemeSelect('dark')}
            role="menuitem"
          >
            <Moon className="mr-2 h-4 w-4" />
            <span>Dark</span>
            {theme === 'dark' && <span className="ml-auto">✓</span>}
          </div>
          <div
            className="relative flex cursor-default select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors focus:bg-accent focus:text-accent-foreground hover:bg-accent hover:text-accent-foreground"
            onClick={() => handleThemeSelect('system')}
            role="menuitem"
          >
            <Monitor className="mr-2 h-4 w-4" />
            <span>System</span>
            {theme === 'system' && <span className="ml-auto">✓</span>}
          </div>
        </div>
      )}
    </div>
  );
};
