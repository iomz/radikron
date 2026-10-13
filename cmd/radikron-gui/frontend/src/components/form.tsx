import React from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '@/lib/utils';

export const fieldClass =
  'h-11 w-full appearance-none rounded-[10px] border border-input bg-card px-3.5 text-foreground outline-none focus-visible:border-ring';

export const primaryButton =
  'min-h-11 cursor-pointer rounded-[10px] border-none bg-primary px-[18px] font-semibold text-primary-foreground disabled:cursor-default disabled:opacity-50';

// dangerButton confirms an action that deletes or forgets something.
export const dangerButton =
  'min-h-11 cursor-pointer rounded-[10px] border-none bg-danger px-[18px] font-semibold text-danger-foreground hover:brightness-110 disabled:cursor-default disabled:opacity-50';

export const secondaryButton =
  'min-h-11 cursor-pointer rounded-[10px] border border-input bg-transparent px-[18px] text-foreground disabled:cursor-default disabled:opacity-50';

// SelectField is a native select drawn like the text fields; WebKit ignores
// the height of a select that keeps its native appearance.
export const SelectField: React.FC<React.SelectHTMLAttributes<HTMLSelectElement>> = ({ className, children, ...props }) => (
  <div className={cn('relative', className)}>
    <select {...props} className={cn(fieldClass, 'cursor-pointer pr-9')}>
      {children}
    </select>
    <ChevronDown aria-hidden="true" className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
  </div>
);
