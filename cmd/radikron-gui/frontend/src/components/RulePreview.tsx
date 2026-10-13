import React, { useEffect, useState } from 'react';
import { useAppStore } from '@/store/useAppStore';
import { formatAirRange } from '@/lib/downloads';
import { programAction } from '@/lib/programs';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Prog, Rule } from '../../bindings/github.com/iomz/radikron/models';

const SHOWN = 30;

// RulePreview lists the programs in radiko's schedule that a rule draft
// matches. It waits for typing to pause before asking the backend.
export const RulePreview: React.FC<{ rule: Rule }> = ({ rule }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const [matches, setMatches] = useState<Prog[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const ready = Boolean(rule.Title.trim() || rule.Pfm.trim() || rule.Keyword.trim());
  // The rule's name and folder do not change what it matches.
  const criteria = JSON.stringify({ ...rule, Name: '', Folder: '' });

  useEffect(() => {
    if (!ready) {
      setMatches(null);
      setError(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const timer = setTimeout(() => {
      App.PreviewRule(rule)
        .then((found) => {
          if (cancelled) return;
          const now = new Date();
          const live = (found ?? []).filter((p): p is Prog => p !== null && programAction(p.Ft, p.To, now) !== 'expired');
          setMatches(live.sort((a, b) => a.Ft.localeCompare(b.Ft)));
          setError(null);
        })
        .catch((err) => {
          if (!cancelled) setError(`Couldn't check radiko's schedule: ${err instanceof Error ? err.message : String(err)}`);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 400);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
    // criteria stands for the parts of rule that matter.
  }, [criteria, ready]);

  const aired = matches?.filter((p) => programAction(p.Ft, p.To) === 'download').length ?? 0;
  let summary: string;
  let note = '';
  if (!ready) summary = 'Fill in a title, host, or keyword to see what this rule matches.';
  else if (error) summary = error;
  else if (matches === null || (loading && matches.length === 0)) summary = "Checking radiko's schedule…";
  else if (matches.length === 0) summary = 'Nothing on radiko matches yet. The rule keeps watching new programs.';
  else {
    const upcoming = matches.length - aired;
    summary = `${matches.length} ${matches.length === 1 ? 'program matches' : 'programs match'}: ${aired} aired, ${upcoming} upcoming.`;
    note = 'After you save, Radikron downloads the aired ones right away and the upcoming ones once they air.';
  }

  return (
    <section aria-label="Matching programs" className="flex flex-col gap-3">
      <h3 className="m-0 text-[15px] font-semibold">Matching programs</h3>
      <span role="status" className={error ? 'text-sm text-state-failed' : 'text-sm text-muted-foreground'}>
        {summary}
      </span>
      {note && <span className="text-xs text-muted-foreground">{note}</span>}
      {ready && !error && matches && matches.length > 0 && (
        <ul className="m-0 flex list-none flex-col divide-y divide-border rounded-[10px] border border-border p-0">
          {matches.slice(0, SHOWN).map((p) => (
            <li key={`${p.StationID}-${p.Ft}`} className="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5">
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className="truncate font-medium">{p.Title}</span>
                <span className="text-xs text-muted-foreground">
                  {stationLabel(p.StationID)} · <span className="font-mono">{formatAirRange(p.Ft, p.To)}</span>
                </span>
              </span>
              <span className="text-xs text-muted-foreground">{programAction(p.Ft, p.To) === 'download' ? 'Aired' : 'Upcoming'}</span>
            </li>
          ))}
          {matches.length > SHOWN && <li className="px-3.5 py-2.5 text-xs text-muted-foreground">and {matches.length - SHOWN} more</li>}
        </ul>
      )}
    </section>
  );
};
