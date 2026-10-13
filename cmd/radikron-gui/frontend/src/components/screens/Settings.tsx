import React, { useEffect, useRef, useState } from 'react';
import { ChevronDown, X } from 'lucide-react';
import { toast } from 'sonner';
import { Browser } from '@wailsio/runtime';
import { useAppStore } from '@/store/useAppStore';
import { getAreaName, regionsData } from '@/lib/regions';
import { ClearHistoryButton, historyCounts } from '@/components/ClearHistoryButton';
import { SelectField, fieldClass, primaryButton, secondaryButton } from '@/components/form';
import { cn } from '@/lib/utils';
import * as App from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import { Settings as SettingsModel } from '../../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const regionGroups: { id: keyof typeof regionsData; label: string }[] = [
  { id: 'hokkaido-tohoku', label: 'Hokkaido and Tohoku' },
  { id: 'kanto', label: 'Kanto' },
  { id: 'hokuriku-koushinetsu', label: 'Hokuriku and Koshinetsu' },
  { id: 'chubu', label: 'Chubu' },
  { id: 'kinki', label: 'Kinki' },
  { id: 'chugoku-shikoku', label: 'Chugoku and Shikoku' },
  { id: 'kyushu', label: 'Kyushu and Okinawa' },
];

const linkClass = 'cursor-pointer border-none bg-transparent p-0 text-signal hover:underline';

// stationPage is the station's page on radiko.
const stationPage = (stationID: string) => `https://radiko.jp/#!/live/${encodeURIComponent(stationID)}`;

const plural = (n: number, noun: string) => `${n} ${noun}${n === 1 ? '' : 's'}`;

const errorText = (err: unknown) => (err instanceof Error ? err.message : String(err));

// Row is one setting: its label and explanation on the left, the control on
// the right. The two stack in a narrow window.
const Row: React.FC<{ id?: string; label: string; hint?: React.ReactNode; children: React.ReactNode }> = ({ id, label, hint, children }) => (
  <div className="grid gap-x-8 gap-y-2 border-t border-border px-5 py-4 first:border-t-0 md:grid-cols-[minmax(0,260px)_minmax(0,1fr)]">
    <div className="flex flex-col gap-1">
      {id ? (
        <label htmlFor={id} className="font-medium">
          {label}
        </label>
      ) : (
        <span className="font-medium">{label}</span>
      )}
      {hint && <span className="text-[13px] text-muted-foreground">{hint}</span>}
    </div>
    <div className="flex min-w-0 flex-col gap-2">{children}</div>
  </div>
);

const Card: React.FC<{ title: string; children: React.ReactNode }> = ({ title, children }) => (
  <section aria-label={title} className="flex flex-col gap-3">
    <h2 className="m-0 text-[15px] font-semibold">{title}</h2>
    <div className="rounded-[14px] border border-border bg-card">{children}</div>
  </section>
);

// StationPicker lists chosen stations as removable chips and adds one from
// the choices.
const StationPicker: React.FC<{
  id: string;
  chosen: string[];
  choices: string[];
  empty: string;
  onChange: (stations: string[]) => void;
}> = ({ id, chosen, choices, empty, onChange }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const byName = (a: string, b: string) => stationLabel(a).localeCompare(stationLabel(b));
  const options = choices.filter((s) => !chosen.includes(s)).sort(byName);
  return (
    <>
      {chosen.length === 0 ? (
        <span className="text-[13px] text-muted-foreground">{empty}</span>
      ) : (
        <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
          {chosen.map((s) => (
            <li key={s} className="flex items-center gap-1 rounded-full border border-border bg-muted py-1 pl-3 pr-1 text-[13px]">
              {stationLabel(s)}
              <button
                type="button"
                aria-label={`Remove ${stationLabel(s)}`}
                onClick={() => onChange(chosen.filter((c) => c !== s))}
                className="grid size-6 cursor-pointer place-items-center rounded-full border-none bg-transparent text-muted-foreground hover:bg-accent hover:text-foreground"
              >
                <X className="size-3.5" aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <SelectField
        id={id}
        value=""
        onChange={(e) => e.target.value && onChange([...chosen, e.target.value])}
        disabled={options.length === 0}
        className="max-w-80"
      >
        <option value="">Add a station…</option>
        {options.map((s) => (
          <option key={s} value={s}>
            {stationLabel(s)}
          </option>
        ))}
      </SelectField>
    </>
  );
};

const logDot: Record<string, string> = {
  info: 'bg-muted-foreground',
  success: 'bg-state-available',
  error: 'bg-state-failed',
};

function logTime(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return timestamp;
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}

// ActivityLog shows what Radikron did since it started, newest at the bottom.
const ActivityLog: React.FC = () => {
  const logs = useAppStore((state) => state.activityLogs);
  const listRef = useRef<HTMLOListElement>(null);
  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [logs]);
  if (logs.length === 0) return <p className="m-0 px-5 py-4 text-[13px] text-muted-foreground">Nothing yet.</p>;
  return (
    <ol ref={listRef} aria-label="Activity log" className="m-0 flex max-h-80 list-none flex-col overflow-y-auto p-0 py-2">
      {logs.map((log) => (
        <li key={log.id} className="grid grid-cols-[auto_8px_minmax(0,1fr)] items-baseline gap-3 px-5 py-1 text-[13px]">
          <span className="font-mono text-xs text-muted-foreground">{logTime(log.timestamp)}</span>
          <span aria-hidden="true" className={cn('size-2 self-center rounded-full', logDot[log.type])} />
          <span className={cn('break-words', log.type === 'error' && 'text-state-failed')}>
            {log.message.replace(/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}\s+/, '')}
          </span>
        </li>
      ))}
    </ol>
  );
};

const sameSettings = (a: SettingsModel | null, b: SettingsModel | null) => JSON.stringify(a) === JSON.stringify(b);

// Settings edits where and how Radikron saves downloads and which stations it
// watches. Changes are saved together to the config file.
export const Settings: React.FC = () => {
  const stations = useAppStore((state) => state.stations);
  const stationLabel = useAppStore((state) => state.stationLabel);
  const refreshStations = useAppStore((state) => state.refreshStations);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const loadStations = useAppStore((state) => state.loadStations);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const history = historyCounts(useAppStore((state) => state.downloads));

  const [saved, setSaved] = useState<SettingsModel | null>(null);
  const [draft, setDraft] = useState<SettingsModel | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [allStations, setAllStations] = useState<string[]>([]);
  const [configFile, setConfigFile] = useState('');
  const [confirmImport, setConfirmImport] = useState(false);

  const load = async () => {
    try {
      const current = await App.GetSettings();
      setSaved(current);
      setDraft(current);
      setLoadError(null);
    } catch (err) {
      setLoadError(`Couldn't load the settings: ${errorText(err)}`);
    }
  };

  useEffect(() => {
    void load();
    App.GetAllStations()
      .then((list) => setAllStations(list ?? []))
      .catch(() => setAllStations([]));
    App.GetConfigFile()
      .then(setConfigFile)
      .catch(() => setConfigFile(''));
  }, []);

  const edit = (fields: Partial<SettingsModel>) => {
    if (!draft) return;
    setDraft(SettingsModel.createFrom({ ...draft, ...fields }));
    setSaveError(null);
  };

  const dirty = draft !== null && !sameSettings(saved, draft);
  const stationsChanged =
    saved !== null &&
    draft !== null &&
    (saved.areaId !== draft.areaId || !sameList(saved.extraStations, draft.extraStations) || !sameList(saved.ignoreStations, draft.ignoreStations));

  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setSaveError(null);
    try {
      await App.SaveSettings(draft);
      await load();
      await Promise.all([loadConfigInfo(), loadStations()]);
      addActivityLog('success', 'Saved settings');
      toast.success('Settings saved');
    } catch (err) {
      setSaveError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const chooseFolder = async () => {
    if (!draft) return;
    try {
      const picked = await App.ChooseDownloadFolder(draft.downloadDir);
      if (picked) edit({ downloadDir: picked });
    } catch (err) {
      toast.error(`Couldn't open the folder picker: ${errorText(err)}`);
    }
  };

  const openFolder = async (path: string) => {
    try {
      await App.OpenDirectory(path);
    } catch (err) {
      toast.error(`Couldn't open the folder: ${errorText(err)}`);
    }
  };

  const exportConfig = async () => {
    try {
      await App.ExportConfigFile();
      toast.success('Exported the config file');
    } catch (err) {
      const message = errorText(err);
      if (!/cancel/i.test(message)) toast.error(`Couldn't export the config file: ${message}`);
    }
  };

  const importConfig = async () => {
    setConfirmImport(false);
    try {
      const imported = await App.ImportConfigFile();
      if (!imported) return;
      setSaveError(null);
      await load();
      await Promise.all([loadConfigInfo(), loadStations()]);
      addActivityLog('success', `Imported settings and rules from ${imported}`);
      toast.success('Imported the config file');
    } catch (err) {
      toast.error(`Couldn't import the config file: ${errorText(err)}`);
    }
  };

  // Stations outside the area that can be added, and the area's stations
  // that can be ignored.
  const watched = draft ? [...new Set([...stations, ...draft.ignoreStations])] : [];
  const extraChoices = allStations.filter((s) => !stations.includes(s) || draft?.extraStations.includes(s));
  const sortedStations = [...stations].sort((a, b) => stationLabel(a).localeCompare(stationLabel(b)));

  return (
    <div className="flex flex-col gap-7">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1.5">
          <h1 className="m-0 text-[28px] font-semibold">Settings</h1>
          {configFile && (
            <span className="text-muted-foreground">
              Saved in <span className="break-all font-mono text-[13px]">{configFile}</span>
            </span>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={secondaryButton} onClick={() => setConfirmImport(true)} disabled={busy}>
            Import config…
          </button>
          <button type="button" className={secondaryButton} onClick={() => void exportConfig()}>
            Export config…
          </button>
        </div>
      </header>

      {confirmImport && (
        <div className="flex flex-wrap items-center gap-3 rounded-xl border border-signal/40 bg-signal/10 px-4 py-3">
          <span className="flex-[1_1_320px]">
            Importing replaces your settings and rules with the ones in the file.
            {dirty && ' Your unsaved changes here are lost.'} Export first to keep a copy.
          </span>
          <button type="button" className={secondaryButton} onClick={() => setConfirmImport(false)}>
            Cancel
          </button>
          <button type="button" className={primaryButton} onClick={() => void importConfig()}>
            Choose a file…
          </button>
        </div>
      )}

      {loadError && (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-3 rounded-xl border border-state-failed/40 bg-state-failed/10 px-4 py-3 text-state-failed"
        >
          <span className="flex-[1_1_320px]">{loadError}</span>
          <button type="button" className={cn(secondaryButton, 'border-state-failed text-state-failed')} onClick={() => void load()}>
            Retry
          </button>
        </div>
      )}

      {draft && (
        <form
          id="settings-form"
          className="flex flex-col gap-7"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <Card title="Downloads">
            <Row label="Download folder" hint="Rules with a folder save inside it unless their folder is a full path.">
              <span className="break-all rounded-lg bg-muted px-3 py-2 font-mono text-xs">{draft.downloadDir}</span>
              <span className="flex flex-wrap gap-2">
                <button type="button" className={secondaryButton} onClick={() => void chooseFolder()}>
                  Choose…
                </button>
                <button type="button" className={secondaryButton} onClick={() => void openFolder(draft.downloadDir)}>
                  Show in {navigator.userAgent.includes('Mac') ? 'Finder' : 'folder'}
                </button>
              </span>
            </Row>
            <Row label="File format" hint="AAC is what radiko sends. MP3 is converted after each download, which takes longer.">
              <div role="radiogroup" aria-label="File format" className="flex w-fit rounded-[10px] border border-input p-1">
                {[
                  { id: 'aac', label: 'AAC' },
                  { id: 'mp3', label: 'MP3' },
                ].map((f) => (
                  <button
                    key={f.id}
                    type="button"
                    role="radio"
                    aria-checked={draft.fileFormat === f.id}
                    onClick={() => edit({ fileFormat: f.id })}
                    className={cn(
                      'min-h-9 min-w-20 cursor-pointer rounded-lg border-none px-4 font-medium',
                      draft.fileFormat === f.id ? 'bg-primary text-primary-foreground' : 'bg-transparent text-muted-foreground hover:text-foreground',
                    )}
                  >
                    {f.label}
                  </button>
                ))}
              </div>
            </Row>
            <Row id="minimum-size" label="Minimum file size" hint="A smaller file is treated as a broken download and tried again.">
              <span className="flex items-center gap-2">
                <input
                  id="minimum-size"
                  type="number"
                  min={0}
                  step={1}
                  value={draft.minimumOutputSizeMb}
                  onChange={(e) => edit({ minimumOutputSizeMb: Math.max(0, Math.trunc(Number(e.target.value) || 0)) })}
                  className={cn(fieldClass, 'w-28')}
                />
                <span className="text-muted-foreground">MB</span>
              </span>
            </Row>
          </Card>

          <Card title="Stations">
            <Row id="area" label="Area" hint="Radikron watches the stations radiko offers in this area.">
              <SelectField id="area" value={draft.areaId} onChange={(e) => edit({ areaId: e.target.value })} className="max-w-80">
                {!getAreaName(draft.areaId) && <option value={draft.areaId}>{draft.areaId}</option>}
                {regionGroups.map((g) => (
                  <optgroup key={g.id} label={g.label}>
                    {regionsData[g.id].map((r) => (
                      <option key={r.id} value={r.id}>
                        {r.name} ({r.id})
                      </option>
                    ))}
                  </optgroup>
                ))}
              </SelectField>
            </Row>
            <Row id="extra-stations" label="Also watch" hint="Stations from other areas.">
              <StationPicker
                id="extra-stations"
                chosen={draft.extraStations}
                choices={extraChoices}
                empty="No stations from other areas."
                onChange={(extraStations) => edit({ extraStations })}
              />
            </Row>
            <Row id="ignore-stations" label="Ignore" hint="Rules and checks leave these stations out.">
              <StationPicker
                id="ignore-stations"
                chosen={draft.ignoreStations}
                choices={watched}
                empty="No stations ignored."
                onChange={(ignoreStations) => edit({ ignoreStations })}
              />
            </Row>
            <Row
              label={`Watching ${stations.length} ${stations.length === 1 ? 'station' : 'stations'}`}
              hint={
                stationsChanged ? (
                  'Save to update this list.'
                ) : (
                  <>
                    Radikron loads the list from radiko when it starts.{' '}
                    <button type="button" className={linkClass} onClick={() => void refreshStations()}>
                      Refresh list
                    </button>
                  </>
                )
              }
            >
              {sortedStations.length === 0 ? (
                <span className="text-[13px]">No stations yet.</span>
              ) : (
                <ul className="flex flex-wrap gap-1.5">
                  {sortedStations.map((s) => (
                    <li key={s}>
                      <a
                        href={stationPage(s)}
                        title={`Open ${stationLabel(s)} on radiko`}
                        onClick={(e) => {
                          e.preventDefault();
                          void Browser.OpenURL(stationPage(s));
                        }}
                        className="block rounded-full border border-border bg-muted px-3 py-1 text-[13px] text-foreground no-underline hover:border-signal hover:text-signal"
                      >
                        {stationLabel(s)}
                      </a>
                    </li>
                  ))}
                </ul>
              )}
            </Row>
          </Card>

          <details className="group flex flex-col gap-3">
            <summary className="flex w-fit cursor-pointer list-none items-center gap-2 text-[15px] font-semibold [&::-webkit-details-marker]:hidden">
              Performance
              <ChevronDown aria-hidden="true" className="size-4 text-muted-foreground transition-transform group-open:rotate-180" />
            </summary>
            <div className="mt-3 rounded-[14px] border border-border bg-card">
              <Row id="max-downloads" label="Downloads at once" hint="How many recording segments Radikron fetches in parallel.">
                <input
                  id="max-downloads"
                  type="number"
                  min={1}
                  value={draft.maxDownloadingConcurrency}
                  onChange={(e) => edit({ maxDownloadingConcurrency: Math.trunc(Number(e.target.value) || 0) })}
                  className={cn(fieldClass, 'w-28')}
                />
              </Row>
              <Row id="max-encodings" label="Conversions at once" hint="Converting is heavy on the processor; keep this low.">
                <input
                  id="max-encodings"
                  type="number"
                  min={1}
                  value={draft.maxEncodingConcurrency}
                  onChange={(e) => edit({ maxEncodingConcurrency: Math.trunc(Number(e.target.value) || 0) })}
                  className={cn(fieldClass, 'w-28')}
                />
              </Row>
            </div>
          </details>

          {dirty && (
            <div className="sticky bottom-4 z-10 flex flex-wrap items-center justify-end gap-3 rounded-[14px] border border-border bg-card px-5 py-3 shadow-lg">
              <span
                role={saveError ? 'alert' : 'status'}
                className={cn('mr-auto text-sm', saveError ? 'text-state-failed' : 'text-muted-foreground')}
              >
                {saveError ?? 'You have unsaved changes.'}
              </span>
              <button
                type="button"
                className={secondaryButton}
                onClick={() => {
                  setDraft(saved);
                  setSaveError(null);
                }}
                disabled={busy}
              >
                Discard
              </button>
              <button type="submit" className={primaryButton} disabled={busy}>
                {busy ? 'Saving…' : 'Save'}
              </button>
            </div>
          )}
        </form>
      )}

      <Card title="History">
        <div className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
          <span className="text-muted-foreground">
            {history.finished + history.skipped === 0
              ? 'Radikron remembers no finished or skipped downloads.'
              : `Radikron remembers ${plural(history.finished, 'finished download')} and ${plural(history.skipped, 'skipped program')}.`}
          </span>
          <ClearHistoryButton className={secondaryButton} />
        </div>
      </Card>

      <Card title="Activity log">
        <ActivityLog />
      </Card>
    </div>
  );
};

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((v, i) => v === b[i]);
}
