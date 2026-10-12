import React, { useEffect, useState } from 'react';
import { X } from 'lucide-react';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import { useProgramSearch } from '@/hooks/useProgramSearch';
import { downloadKey, formatAirRange } from '@/lib/downloads';
import { dayParts, filterResults, programAction, suggestRuleName, type DayPart, type ProgramAction } from '@/lib/programs';
import { broadcastDay, formatDay, todayBroadcastDay } from '@/lib/timetable';
import { SanitizedHTML } from '@/components/SanitizedHTML';
import { SelectField, fieldClass, primaryButton, secondaryButton } from '@/components/form';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import * as radikron from '../../../bindings/github.com/iomz/radikron/models';

// searchDays lists the broadcast days radiko's schedule covers: the past week
// and the week ahead.
function searchDays(now: Date): string[] {
  const today = todayBroadcastDay(now);
  const base = Date.UTC(Number(today.slice(0, 4)), Number(today.slice(4, 6)) - 1, Number(today.slice(6, 8)));
  return Array.from({ length: 14 }, (_, i) => {
    const d = new Date(base + (i - 6) * 86_400_000);
    return `${d.getUTCFullYear()}${String(d.getUTCMonth() + 1).padStart(2, '0')}${String(d.getUTCDate()).padStart(2, '0')}`;
  });
}

const actionLabels: Record<ProgramAction, { button: string; dialog: string; confirm: string; done: string }> = {
  download: { button: 'Download now', dialog: 'Download now', confirm: 'Download', done: 'Downloading' },
  schedule: { button: 'Schedule download', dialog: 'Schedule download', confirm: 'Schedule', done: 'Scheduled' },
  expired: { button: '', dialog: '', confirm: '', done: '' },
};

// Search finds programs in radiko's weekly schedule and downloads, schedules,
// or turns one into a rule.
export const Search: React.FC = () => {
  const stationsRaw = useAppStore((state) => state.stations);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const downloads = useAppStore((state) => state.downloads);
  const configInfo = useAppStore((state) => state.configInfo);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const addRule = useAppStore((state) => state.addRule);
  const loadDownloads = useAppStore((state) => state.loadDownloads);
  const { initializing, loading, error, results, submitted, search, clear } = useProgramSearch();

  const [keyword, setKeyword] = useState('');
  const [station, setStation] = useState('');
  const [day, setDay] = useState('');
  const [part, setPart] = useState<DayPart | ''>('');
  const [expanded, setExpanded] = useState<string | null>(null);
  const [saving, setSaving] = useState<radikron.Prog | null>(null);
  const [ruleName, setRuleName] = useState('');
  const [folder, setFolder] = useState('');
  const [busy, setBusy] = useState(false);
  const [ruleDraft, setRuleDraft] = useState<radikron.Rule | null>(null);
  const [ruleError, setRuleError] = useState<string | null>(null);

  const stations = [...(stationsRaw ?? [])].sort((a, b) => stationLabel(a).localeCompare(stationLabel(b)));
  const rules = (configInfo?.Rules ?? []).filter((r): r is radikron.Rule => r !== null);
  const today = todayBroadcastDay(new Date());
  const shown = filterResults(results, { day, part }, broadcastDay);
  const narrowed = Boolean(station || day || part);
  const scheduledKeys = new Set((downloads?.scheduled ?? []).map((d) => downloadKey(d.stationId, d.start)));

  // Preview where the download will be saved for the chosen rule.
  useEffect(() => {
    if (!saving) return;
    const rule = rules.find((r) => r.Name === ruleName);
    const downloadDir = configInfo?.DownloadDir || 'radiko';
    App.GetDesignatedFolder(saving, rule?.Folder ?? '', downloadDir)
      .then(setFolder)
      .catch(() => setFolder(rule?.Folder ? `${downloadDir}/${rule.Folder}` : downloadDir));
    // Rules and the download folder change rarely; re-run when the choice does.
  }, [saving, ruleName]);

  const openSave = (prog: radikron.Prog) => {
    setRuleName(suggestRuleName(rules, prog));
    setFolder('');
    setSaving(prog);
  };

  const confirmSave = async () => {
    if (!saving) return;
    const action = programAction(saving.Ft, saving.To);
    setBusy(true);
    try {
      await App.InjectProgram(saving, ruleName);
      const message = `${actionLabels[action].done} ${saving.Title} (${stationLabel(saving.StationID)})`;
      addActivityLog('success', message);
      toast.success(message);
      setSaving(null);
      await loadDownloads();
    } catch (err) {
      toast.error(`Couldn't add it: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  const openRule = async (prog: radikron.Prog) => {
    setRuleError(null);
    try {
      setRuleDraft(await App.RuleFromProgram(prog));
    } catch (err) {
      toast.error(`Couldn't prepare a rule: ${err instanceof Error ? err.message : String(err)}`);
    }
  };

  const confirmRule = async () => {
    if (!ruleDraft) return;
    setBusy(true);
    setRuleError(null);
    try {
      await addRule(ruleDraft);
      toast.success(`Added rule "${ruleDraft.Name.trim()}"`);
      setRuleDraft(null);
    } catch (err) {
      setRuleError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const savingAction = saving ? programAction(saving.Ft, saving.To) : 'schedule';

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-1.5">
        <h1 className="m-0 text-[28px] font-semibold">Search programs</h1>
        <span className="text-muted-foreground">Searches radiko's schedule for the past week and the week ahead.</span>
      </header>

      <form
        className="flex flex-wrap items-end gap-2.5"
        onSubmit={(e) => {
          e.preventDefault();
          setExpanded(null);
          void search(keyword, station, narrowed);
        }}
      >
        <div className="flex flex-[1_1_260px] flex-col gap-1.5">
          <label htmlFor="keyword" className="text-xs text-muted-foreground">
            Keyword
          </label>
          <div className="relative">
            <input
              id="keyword"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder="MIDDAY LOUNGE"
              className={cn(fieldClass, 'pr-10')}
            />
            {keyword && (
              <button
                type="button"
                aria-label="Clear search"
                onClick={() => {
                  setKeyword('');
                  setExpanded(null);
                  clear();
                }}
                className="absolute right-2 top-1/2 grid size-7 -translate-y-1/2 cursor-pointer place-items-center rounded-full border-none bg-transparent text-muted-foreground hover:bg-accent hover:text-foreground"
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            )}
          </div>
        </div>
        <div className="flex flex-[0_1_190px] flex-col gap-1.5">
          <label htmlFor="station" className="text-xs text-muted-foreground">
            Station
          </label>
          <SelectField id="station" value={station} onChange={(e) => setStation(e.target.value)}>
            <option value="">All stations</option>
            {stations.map((s) => (
              <option key={s} value={s}>
                {stationLabel(s)}
              </option>
            ))}
          </SelectField>
        </div>
        <div className="flex flex-[0_1_160px] flex-col gap-1.5">
          <label htmlFor="day" className="text-xs text-muted-foreground">
            Day
          </label>
          <SelectField id="day" value={day} onChange={(e) => setDay(e.target.value)}>
            <option value="">Any day</option>
            {searchDays(new Date()).map((d) => (
              <option key={d} value={d}>
                {formatDay(d)}
                {d === today ? ' (today)' : ''}
              </option>
            ))}
          </SelectField>
        </div>
        <div className="flex flex-[0_1_180px] flex-col gap-1.5">
          <label htmlFor="part" className="text-xs text-muted-foreground">
            Time
          </label>
          <SelectField id="part" value={part} onChange={(e) => setPart(e.target.value as DayPart | '')}>
            <option value="">Any time</option>
            {dayParts.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </SelectField>
        </div>
        <button type="submit" disabled={initializing || (!keyword.trim() && !narrowed)} className={primaryButton}>
          {loading ? 'Searching…' : 'Search'}
        </button>
      </form>

      {error && (
        <div role="alert" className="rounded-xl border border-state-failed/40 bg-state-failed/10 px-4 py-3 text-state-failed">
          {error}
        </div>
      )}

      {initializing ? (
        <p className="m-0 text-muted-foreground">Loading this week's programs…</p>
      ) : submitted === null ? (
        <p className="m-0 text-muted-foreground">
          Enter words to search titles, hosts, and descriptions, or pick a station or day to browse. Only programs radiko still has are shown.
        </p>
      ) : !loading && shown.length === 0 && !error ? (
        <p className="m-0 text-muted-foreground">No programs found. Try other words or filters.</p>
      ) : shown.length > 0 ? (
        <span className="text-[13px] text-muted-foreground">
          {shown.length} {shown.length === 1 ? 'program' : 'programs'}
          {submitted ? ` for “${submitted}”` : ''}
        </span>
      ) : null}

      <div className="flex flex-col gap-3">
        {shown.map((prog) => {
          const key = downloadKey(prog.StationID, prog.Ft);
          const open = expanded === key;
          const action = programAction(prog.Ft, prog.To);
          const scheduled = scheduledKeys.has(key);
          return (
            <article key={prog.ID || key} className={cn('rounded-[14px] border bg-card', open ? 'border-signal' : 'border-border')}>
              <button
                type="button"
                aria-expanded={open}
                onClick={() => setExpanded(open ? null : key)}
                className="flex w-full cursor-pointer flex-wrap items-center justify-between gap-3 border-none bg-transparent px-5 py-4 text-left text-foreground"
              >
                <span className="flex min-w-0 flex-col gap-1">
                  <span className={cn('font-medium', open && 'text-lg font-semibold')}>{prog.Title}</span>
                  <span className="text-[13px] text-muted-foreground">
                    {stationLabel(prog.StationID)} · <span className="font-mono">{formatAirRange(prog.Ft, prog.To)}</span>
                  </span>
                </span>
                <span className="text-[13px] text-muted-foreground">
                  {scheduled ? 'In Downloads' : action === 'download' ? 'Ready to download' : 'Upcoming'}
                </span>
              </button>
              {open && (
                <div className="flex flex-col gap-3 px-5 pb-[18px]">
                  {prog.Pfm && <span className="text-muted-foreground">Host: {prog.Pfm}</span>}
                  {prog.Desc && <SanitizedHTML html={prog.Desc} className="leading-relaxed text-foreground/85 [&_a]:text-signal [&_a]:underline" />}
                  <div className="flex flex-wrap gap-2.5 pt-1">
                    {action !== 'expired' && !scheduled && (
                      <button type="button" className={primaryButton} onClick={() => openSave(prog)}>
                        {actionLabels[action].button}
                      </button>
                    )}
                    <button type="button" className={secondaryButton} onClick={() => void openRule(prog)}>
                      Create rule for every episode…
                    </button>
                  </div>
                </div>
              )}
            </article>
          );
        })}
      </div>

      <Dialog open={saving !== null} onOpenChange={(open) => !open && setSaving(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{actionLabels[savingAction].dialog}</DialogTitle>
            <DialogDescription>
              {saving && `${saving.Title} · ${stationLabel(saving.StationID)} · ${formatAirRange(saving.Ft, saving.To)}`}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="save-rule" className="text-xs text-muted-foreground">
                Save with the folder of rule
              </label>
              <SelectField id="save-rule" value={ruleName} onChange={(e) => setRuleName(e.target.value)}>
                <option value="">No rule (download folder)</option>
                {rules.map((r) => (
                  <option key={r.Name} value={r.Name}>
                    {r.Name}
                    {r.Folder ? ` (${r.Folder})` : ''}
                  </option>
                ))}
              </SelectField>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">Saves to</span>
              <span className="break-all rounded-lg bg-muted px-3 py-2 font-mono text-xs">{folder || '…'}</span>
            </div>
            {savingAction === 'schedule' && (
              <span className="text-xs text-muted-foreground">Radikron downloads it after the program ends, while the app is open.</span>
            )}
            <div className="flex justify-end gap-2">
              <button type="button" className={secondaryButton} onClick={() => setSaving(null)} disabled={busy}>
                Cancel
              </button>
              <button type="button" className={primaryButton} onClick={() => void confirmSave()} disabled={busy}>
                {busy ? 'Working…' : actionLabels[savingAction].confirm}
              </button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={ruleDraft !== null} onOpenChange={(open) => !open && setRuleDraft(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Create rule</DialogTitle>
            <DialogDescription>Radikron downloads every episode the rule matches, including episodes from the past week.</DialogDescription>
          </DialogHeader>
          {ruleDraft && (
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                void confirmRule();
              }}
            >
              <div className="flex flex-col gap-1.5">
                <label htmlFor="rule-name" className="text-xs text-muted-foreground">
                  Rule name
                </label>
                <input
                  id="rule-name"
                  value={ruleDraft.Name}
                  onChange={(e) => setRuleDraft(radikron.Rule.createFrom({ ...ruleDraft, Name: e.target.value }))}
                  className={fieldClass}
                />
              </div>
              <span className="text-sm text-muted-foreground">
                Matches programs titled “{ruleDraft.Title}”{ruleDraft.StationID ? ` on ${stationLabel(ruleDraft.StationID)}` : ''}. You can edit it
                later in Rules.
              </span>
              {ruleError && <span className="text-sm text-state-failed">{ruleError}</span>}
              <div className="flex justify-end gap-2">
                <button type="button" className={secondaryButton} onClick={() => setRuleDraft(null)} disabled={busy}>
                  Cancel
                </button>
                <button type="submit" className={primaryButton} disabled={busy || !ruleDraft.Name.trim()}>
                  {busy ? 'Creating…' : 'Create rule'}
                </button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
};
