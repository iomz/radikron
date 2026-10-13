import { useCallback, useEffect, useRef, useState } from 'react';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Prog } from '../../bindings/github.com/iomz/radikron/models';

// useProgramSearch searches radiko's weekly programs. It loads the program
// snapshots once, searches only when asked, and ignores responses from
// searches that a newer one replaced.
export function useProgramSearch() {
  const [initializing, setInitializing] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [results, setResults] = useState<Prog[]>([]);
  // The keyword of the last search ("" when browsing); null before the first.
  const [submitted, setSubmitted] = useState<string | null>(null);
  const requestRef = useRef(0);

  useEffect(() => {
    let cancelled = false;
    App.FetchProgramSnapshots()
      .catch((err) => {
        if (!cancelled) setError(`Couldn't load this week's programs: ${err instanceof Error ? err.message : String(err)}`);
      })
      .finally(() => {
        if (!cancelled) setInitializing(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // search runs a search. Without a keyword it lists every program, which is
  // only useful when the caller narrows the results some other way (browse).
  const search = useCallback(async (keyword: string, station: string, browse = false) => {
    const trimmed = keyword.trim();
    const request = ++requestRef.current;
    setError(null);
    if (!trimmed && !browse) {
      setSubmitted(null);
      setResults([]);
      return;
    }
    setSubmitted(trimmed);
    setLoading(true);
    try {
      const found = (await App.SearchWeeklyPrograms('', '', trimmed, station)) ?? [];
      if (request !== requestRef.current) return;
      const progs = found.filter((p): p is Prog => p !== null);
      setResults(progs.sort((a, b) => a.Ft.localeCompare(b.Ft)));
    } catch (err) {
      if (request !== requestRef.current) return;
      setResults([]);
      setError(`Search failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      if (request === requestRef.current) setLoading(false);
    }
  }, []);

  const clear = useCallback(() => {
    requestRef.current++;
    setSubmitted(null);
    setResults([]);
    setError(null);
    setLoading(false);
  }, []);

  return { initializing, loading, error, results, submitted, search, clear };
}
