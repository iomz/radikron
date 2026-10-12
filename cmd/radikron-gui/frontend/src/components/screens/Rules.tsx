import React, { useState } from 'react';
import { ChevronDown, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { useAppStore } from '@/store/useAppStore';
import { downloadsList, formatAirTime } from '@/lib/downloads';
import { describeRule, timestampRuleName, ruleActivity, ruleProblem, weekdays, windowOptions } from '@/lib/rules';
import type { ScreenId } from '@/lib/navigation';
import { RulePreview } from '@/components/RulePreview';
import { SelectField, fieldClass, primaryButton, secondaryButton } from '@/components/form';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import * as radikron from '../../../bindings/github.com/iomz/radikron/models';

function emptyRule(name: string): radikron.Rule {
  return radikron.Rule.createFrom({
    Name: name,
    Title: '',
    Pfm: '',
    Keyword: '',
    StationID: '',
    DoW: [],
    Window: '',
    Folder: '',
  });
}

// isAbsolute reports whether folder is an absolute path on macOS, Linux, or
// Windows; such a rule folder is used as is instead of inside the download folder.
function isAbsolute(folder: string): boolean {
  return /^([/\\]|[A-Za-z]:[/\\])/.test(folder);
}

function joinPath(dir: string, folder: string): string {
  if (!folder) return dir;
  if (isAbsolute(folder)) return folder;
  return `${dir.replace(/[\\/]+$/, '')}/${folder}`;
}

interface Draft {
  // The rule's name before editing; null for a new rule.
  original: string | null;
  rule: radikron.Rule;
}

const Field: React.FC<{
  id: string;
  label: string;
  hint?: string;
  className?: string;
  children: React.ReactNode;
}> = ({ id, label, hint, className, children }) => (
  <div className={cn('flex flex-col gap-1.5', className)}>
    <label htmlFor={id} className="text-xs text-muted-foreground">
      {label}
    </label>
    {children}
    {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
  </div>
);

function skipSummary(exclude: radikron.Criteria | null | undefined): string {
  const set = [exclude?.Title, exclude?.Pfm, exclude?.Keyword].filter((v) => v?.trim());
  return set.length === 0 ? 'None' : set.map((v) => `“${v}”`).join(', ');
}

// Section is a collapsible part of the rule form, closed until opened, with
// its current value shown in the header.
const Section: React.FC<{ title: string; summary: string; mono?: boolean; children: React.ReactNode }> = ({ title, summary, mono, children }) => (
  <details className="group rounded-[10px] border border-border">
    <summary className="flex min-h-11 cursor-pointer list-none items-center gap-3 px-4 py-2.5 [&::-webkit-details-marker]:hidden">
      <span className="text-[15px] font-semibold">{title}</span>
      <span className={cn('min-w-0 flex-1 truncate text-right text-xs text-muted-foreground', mono && 'font-mono')}>{summary}</span>
      <ChevronDown aria-hidden="true" className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" />
    </summary>
    <div className="flex flex-col gap-3 px-4 pb-4 pt-1">{children}</div>
  </details>
);

// Rules lists the download rules and edits them one at a time.
export const Rules: React.FC<{ onNavigate: (screen: ScreenId) => void }> = ({ onNavigate }) => {
  const configInfo = useAppStore((state) => state.configInfo);
  const stationsRaw = useAppStore((state) => state.stations);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const downloads = useAppStore((state) => state.downloads);
  const addRule = useAppStore((state) => state.addRule);
  const updateRule = useAppStore((state) => state.updateRule);
  const removeRule = useAppStore((state) => state.removeRule);

  const [draft, setDraft] = useState<Draft | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const rules = (configInfo?.Rules ?? []).filter((r): r is radikron.Rule => r !== null);
  const items = downloadsList(downloads);
  const downloadDir = configInfo?.DownloadDir || 'radiko';
  const stations = [...(stationsRaw ?? [])].sort((a, b) => stationLabel(a).localeCompare(stationLabel(b)));

  const open = (original: string | null, rule: radikron.Rule) => {
    setSaveError(null);
    setConfirmDelete(false);
    setDraft({ original, rule: radikron.Rule.createFrom(rule) });
  };

  const edit = (fields: Partial<radikron.Rule>) => {
    if (!draft) return;
    setSaveError(null);
    setDraft({
      ...draft,
      rule: radikron.Rule.createFrom({ ...draft.rule, ...fields }),
    });
  };

  const editExclude = (fields: Partial<radikron.Criteria>) => {
    if (!draft) return;
    edit({
      Exclude: radikron.Criteria.createFrom({
        ...(draft.rule.Exclude ?? {}),
        ...fields,
      }),
    });
  };

  const toggleDay = (day: string) => {
    if (!draft) return;
    const days = (draft.rule.DoW ?? []).map((d) => d.toLowerCase());
    edit({
      DoW: days.includes(day) ? days.filter((d) => d !== day) : [...days, day],
    });
  };

  const chooseFolder = async () => {
    if (!draft) return;
    try {
      const folder = await App.ChooseRuleFolder(draft.rule.Folder);
      if (folder !== null) edit({ Folder: folder });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setSaveError(null);
    try {
      if (draft.original === null) {
        await addRule(draft.rule);
        toast.success(`Added rule "${draft.rule.Name.trim()}"`);
      } else {
        await updateRule(draft.original, draft.rule);
        toast.success(`Saved rule "${draft.rule.Name.trim()}"`);
      }
      setDraft(null);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!draft?.original) return;
    setBusy(true);
    setSaveError(null);
    try {
      await removeRule(draft.original);
      toast.success(`Removed rule "${draft.original}"`);
      setDraft(null);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
      setConfirmDelete(false);
    } finally {
      setBusy(false);
    }
  };

  const rule = draft?.rule;
  const problem = rule ? ruleProblem(rule, rules, draft.original) : null;
  const ruleStations = rule?.StationID && !stations.includes(rule.StationID) ? [rule.StationID, ...stations] : stations;
  const customWindow = rule?.Window && !windowOptions.some((w) => w.value === rule.Window) ? rule.Window : null;
  const exclude = rule?.Exclude;

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1.5">
          <h1 className="m-0 text-[28px] font-semibold">Rules</h1>
          <span className="text-muted-foreground">Radikron downloads every program that matches a rule, as soon as radiko has it.</span>
        </div>
        <button
          type="button"
          className={cn(primaryButton, 'inline-flex items-center gap-2')}
          onClick={() => open(null, emptyRule(timestampRuleName(rules)))}
          disabled={!configInfo}
        >
          <Plus className="size-4" aria-hidden="true" />
          New rule
        </button>
      </header>

      {rules.length === 0 ? (
        <section className="flex flex-col items-start gap-3 rounded-[14px] border border-border bg-card px-5 py-[18px]">
          <span className="font-medium">No rules yet</span>
          <span className="text-muted-foreground">Create a rule here, or find a program in Search and choose “Create rule for every episode”.</span>
          <button type="button" className={secondaryButton} onClick={() => onNavigate('search')}>
            Go to Search
          </button>
        </section>
      ) : (
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {rules.map((r) => {
            const { upcoming, next } = ruleActivity(r.Name, items);
            return (
              <li key={r.Name}>
                <button
                  type="button"
                  onClick={() => open(r.Name, r)}
                  className="flex w-full cursor-pointer flex-wrap items-center justify-between gap-3 rounded-[14px] border border-border bg-card px-5 py-4 text-left text-foreground hover:border-signal"
                >
                  <span className="flex min-w-0 flex-col gap-1">
                    <span className="font-medium">{r.Name}</span>
                    <span className="text-[13px] text-muted-foreground">{describeRule(r, stationLabel).join(' · ')}</span>
                    <span className="break-all font-mono text-xs text-muted-foreground">{joinPath(downloadDir, r.Folder)}</span>
                  </span>
                  <span className="flex flex-col items-end gap-1 text-[13px] text-muted-foreground">
                    <span>{upcoming === 0 ? 'Nothing scheduled' : `${upcoming} scheduled`}</span>
                    {next && (
                      <span className="max-w-64 truncate">
                        Next: <span className="font-mono">{formatAirTime(next.start)}</span> {next.title}
                      </span>
                    )}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}

      <Dialog open={draft !== null} onOpenChange={(isOpen) => !isOpen && !busy && setDraft(null)}>
        <DialogContent className="flex max-h-[88vh] w-[calc(100vw-32px)] max-w-[1040px] flex-col gap-0 overflow-hidden p-0">
          <DialogHeader className="border-b border-border px-6 pb-4 pt-6">
            <DialogTitle>{draft?.original === null ? 'New rule' : 'Edit rule'}</DialogTitle>
            <DialogDescription>
              A program must match everything you fill in. Text must match exactly, including upper and lower case.
            </DialogDescription>
          </DialogHeader>
          {rule && (
            <div className="grid min-h-0 flex-1 overflow-y-auto md:grid-cols-[minmax(0,1fr)_minmax(0,380px)] md:overflow-hidden">
              <form
                id="rule-form"
                className="flex flex-col gap-6 px-6 py-5 md:overflow-y-auto"
                onSubmit={(e) => {
                  e.preventDefault();
                  void save();
                }}
              >
                <Field id="rule-name" label="Name">
                  <input id="rule-name" value={rule.Name} onChange={(e) => edit({ Name: e.target.value })} className={fieldClass} />
                </Field>

                <fieldset className="m-0 flex flex-col gap-4 border-none p-0">
                  <legend className="mb-3 p-0 text-[15px] font-semibold">Match</legend>
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <Field id="rule-title" label="Title">
                      <input
                        id="rule-title"
                        value={rule.Title}
                        onChange={(e) => edit({ Title: e.target.value })}
                        placeholder="MIDDAY LOUNGE"
                        className={fieldClass}
                      />
                    </Field>
                    <Field id="rule-pfm" label="Host">
                      <input
                        id="rule-pfm"
                        value={rule.Pfm}
                        onChange={(e) => edit({ Pfm: e.target.value })}
                        placeholder="クリス・ペプラー"
                        className={fieldClass}
                      />
                    </Field>
                    <Field id="rule-keyword" label="Keyword" hint="Searches the title, host, and description." className="sm:col-span-2">
                      <input
                        id="rule-keyword"
                        value={rule.Keyword}
                        onChange={(e) => edit({ Keyword: e.target.value })}
                        placeholder="jazz"
                        className={fieldClass}
                      />
                    </Field>
                  </div>
                </fieldset>

                <fieldset className="m-0 flex flex-col gap-4 border-none p-0">
                  <legend className="mb-3 p-0 text-[15px] font-semibold">Limit to</legend>
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <Field id="rule-station" label="Station">
                      <SelectField id="rule-station" value={rule.StationID} onChange={(e) => edit({ StationID: e.target.value })}>
                        <option value="">Any station</option>
                        {ruleStations.map((s) => (
                          <option key={s} value={s}>
                            {stationLabel(s)}
                          </option>
                        ))}
                      </SelectField>
                    </Field>
                    <Field id="rule-window" label="Past episodes">
                      <SelectField id="rule-window" value={rule.Window} onChange={(e) => edit({ Window: e.target.value })}>
                        {windowOptions.map((w) => (
                          <option key={w.value} value={w.value}>
                            {w.label}
                          </option>
                        ))}
                        {customWindow && <option value={customWindow}>Last {customWindow}</option>}
                      </SelectField>
                    </Field>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <span id="rule-days" className="text-xs text-muted-foreground">
                      Days
                    </span>
                    <div role="group" aria-labelledby="rule-days" className="flex flex-wrap gap-2">
                      {weekdays.map((d) => {
                        const on = (rule.DoW ?? []).some((v) => v.toLowerCase() === d.id);
                        return (
                          <button
                            key={d.id}
                            type="button"
                            aria-pressed={on}
                            onClick={() => toggleDay(d.id)}
                            className={cn(
                              'min-h-9 min-w-12 cursor-pointer rounded-full px-3.5 text-[13px]',
                              on ? 'border border-signal bg-signal/12 text-signal' : 'border border-input bg-transparent text-muted-foreground',
                            )}
                          >
                            {d.label}
                          </button>
                        );
                      })}
                    </div>
                    <span className="text-xs text-muted-foreground">
                      None selected means every day. Days follow the clock, so a program at 01:00 on Monday counts as Monday.
                    </span>
                  </div>
                </fieldset>

                <div className="flex flex-col gap-3">
                  <Section title="Folder" summary={joinPath(downloadDir, rule.Folder)} mono>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="min-h-11 flex-[1_1_240px] content-center break-all rounded-[10px] bg-muted px-3.5 py-2 font-mono text-xs">
                        {joinPath(downloadDir, rule.Folder)}
                      </span>
                      <button type="button" className={secondaryButton} onClick={() => void chooseFolder()} disabled={busy}>
                        Choose…
                      </button>
                    </div>
                    {rule.Folder && (
                      <button
                        type="button"
                        onClick={() => edit({ Folder: '' })}
                        className="w-fit cursor-pointer border-none bg-transparent p-0 text-xs text-signal underline"
                      >
                        Use the download folder
                      </button>
                    )}
                  </Section>

                  <Section title="Skip programs" summary={skipSummary(exclude)}>
                    <span className="text-xs text-muted-foreground">Skips a program if any of these match.</span>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <Field id="skip-title" label="Title">
                        <input
                          id="skip-title"
                          value={exclude?.Title ?? ''}
                          onChange={(e) => editExclude({ Title: e.target.value })}
                          className={fieldClass}
                        />
                      </Field>
                      <Field id="skip-pfm" label="Host">
                        <input
                          id="skip-pfm"
                          value={exclude?.Pfm ?? ''}
                          onChange={(e) => editExclude({ Pfm: e.target.value })}
                          className={fieldClass}
                        />
                      </Field>
                      <Field id="skip-keyword" label="Keyword" className="sm:col-span-2">
                        <input
                          id="skip-keyword"
                          value={exclude?.Keyword ?? ''}
                          onChange={(e) => editExclude({ Keyword: e.target.value })}
                          placeholder="再放送"
                          className={fieldClass}
                        />
                      </Field>
                    </div>
                  </Section>
                </div>
              </form>

              <aside className="border-t border-border bg-muted/40 px-6 py-5 md:overflow-y-auto md:border-l md:border-t-0">
                <RulePreview rule={rule} />
              </aside>
            </div>
          )}

          <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-6 py-4">
            {draft?.original !== null ? (
              confirmDelete ? (
                <span className="flex flex-wrap items-center gap-2 text-sm">
                  Delete “{draft?.original}”?
                  <button type="button" className={secondaryButton} onClick={() => setConfirmDelete(false)} disabled={busy}>
                    Keep
                  </button>
                  <button type="button" className={cn(primaryButton, 'bg-state-failed text-white')} onClick={() => void remove()} disabled={busy}>
                    {busy ? 'Deleting…' : 'Delete'}
                  </button>
                </span>
              ) : (
                <button
                  type="button"
                  className={cn(secondaryButton, 'border-state-failed/50 text-state-failed')}
                  onClick={() => setConfirmDelete(true)}
                  disabled={busy}
                >
                  Delete rule
                </button>
              )
            ) : (
              <span />
            )}
            {!confirmDelete && (
              <span className="flex flex-wrap items-center justify-end gap-3">
                {(saveError || problem) && (
                  <span role={saveError ? 'alert' : undefined} className={cn('text-sm', saveError ? 'text-state-failed' : 'text-muted-foreground')}>
                    {saveError ?? problem}
                  </span>
                )}
                <button type="button" className={secondaryButton} onClick={() => setDraft(null)} disabled={busy}>
                  Cancel
                </button>
                <button type="submit" form="rule-form" className={primaryButton} disabled={busy || problem !== null}>
                  {busy ? 'Saving…' : draft?.original === null ? 'Create rule' : 'Save'}
                </button>
              </span>
            )}
          </footer>
        </DialogContent>
      </Dialog>
    </div>
  );
};
