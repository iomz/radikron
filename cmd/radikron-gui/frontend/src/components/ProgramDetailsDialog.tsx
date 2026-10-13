import React, { useEffect, useState } from 'react';
import { useAppStore } from '@/store/useAppStore';
import { downloadKey, downloadNote, downloadSource } from '@/lib/downloads';
import { StateChip } from '@/components/StateChip';
import { DownloadActionButton } from '@/components/DownloadActionButton';
import { SanitizedHTML } from '@/components/SanitizedHTML';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import * as App from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/app';
import type { Prog } from '../../bindings/github.com/iomz/radikron/models';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

// formatRange formats radiko times as "2026/10/12 13:00–14:00".
export function formatRange(ft: string, to: string): string {
  const t = (v: string) => (/^\d{14}$/.test(v) ? `${v.slice(8, 10)}:${v.slice(10, 12)}` : '');
  if (!/^\d{14}$/.test(ft)) return '';
  const end = t(to);
  return `${ft.slice(0, 4)}/${ft.slice(4, 6)}/${ft.slice(6, 8)} ${t(ft)}${end ? `–${end}` : ''}`;
}

const htmlClass = 'leading-relaxed [&_a]:text-signal [&_a]:underline';

// ProgramDetailsDialog shows a download's state and, while the program is
// still scheduled, radiko's description of it.
export const ProgramDetailsDialog: React.FC<{ item: DownloadItem | null; onClose: () => void }> = ({ item, onClose }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const [prog, setProg] = useState<Prog | null>(null);

  useEffect(() => {
    setProg(null);
    if (!item) return;
    let cancelled = false;
    App.GetSchedules()
      .then((schedules) => {
        const found = (schedules ?? []).find((p) => p && downloadKey(p.StationID, p.Ft) === downloadKey(item.stationId, item.start));
        if (!cancelled && found) setProg(found);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [item]);

  return (
    <Dialog open={item !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[80vh] max-w-2xl overflow-y-auto">
        {item && (
          <>
            <DialogHeader>
              <DialogTitle>{item.title}</DialogTitle>
              <DialogDescription>
                {stationLabel(item.stationId)}
                {prog?.Pfm && ` · ${prog.Pfm}`}
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 text-sm">
              <span className="font-mono text-muted-foreground">{formatRange(item.start, item.end)}</span>
              <div className="flex flex-wrap items-center gap-2">
                <StateChip state={item.state} />
                <span className="text-muted-foreground">
                  {downloadNote(item)} · {downloadSource(item)}
                </span>
                <DownloadActionButton item={item} className="ml-auto" onDone={onClose} />
              </div>
              {prog?.Desc && <SanitizedHTML html={prog.Desc} className={htmlClass} />}
              {prog?.Info && <SanitizedHTML html={prog.Info} className={htmlClass} />}
              {prog?.Tags && prog.Tags.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {prog.Tags.map((tag) => (
                    <span key={tag} className="rounded-full bg-muted px-2.5 py-0.5 text-xs text-muted-foreground">
                      {tag}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
};
