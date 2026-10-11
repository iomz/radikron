import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppStore } from '@/store/useAppStore';
import {
  downloadDetail,
  downloadStateLabel,
  formatProgramTime,
  nextDownloads,
} from '@/lib/downloads';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

const SUMMARY_LIMIT = 5;

const DownloadRow: React.FC<{ item: DownloadItem }> = ({ item }) => {
  const stationLabel = useAppStore((state) => state.stationLabel);
  const detail = downloadDetail(item);
  return (
    <li className="text-sm">
      <span className="text-muted-foreground">{formatProgramTime(item.start)}</span>{' '}
      <span>{stationLabel(item.stationId)}</span> <span className="font-medium">{item.title}</span>{' '}
      <span className="text-muted-foreground">
        · {downloadStateLabel(item.state)}
        {item.manual ? ' · manual' : item.ruleName ? ` · rule ${item.ruleName}` : ''}
      </span>
      {detail && <div className="text-xs text-muted-foreground">{detail}</div>}
    </li>
  );
};

// DownloadSummary lists upcoming downloads and this session's results. The
// layout is provisional until the GUI redesign (#77).
export const DownloadSummary: React.FC = () => {
  const downloads = useAppStore((state) => state.downloads);
  const next = nextDownloads(downloads, SUMMARY_LIMIT);
  const recent = (downloads?.recent ?? []).slice(0, SUMMARY_LIMIT);

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>Next downloads</CardTitle>
          <CardDescription>Programs your rules matched that are not downloaded yet</CardDescription>
        </CardHeader>
        <CardContent>
          {next.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nothing scheduled</p>
          ) : (
            <ul className="space-y-2">
              {next.map((item) => (
                <DownloadRow key={`${item.stationId}|${item.start}`} item={item} />
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>Recent results</CardTitle>
          <CardDescription>Downloads finished since Radikron started</CardDescription>
        </CardHeader>
        <CardContent>
          {recent.length === 0 ? (
            <p className="text-sm text-muted-foreground">No downloads finished yet</p>
          ) : (
            <ul className="space-y-2">
              {recent.map((item) => (
                <DownloadRow key={`${item.stationId}|${item.start}`} item={item} />
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </>
  );
};
