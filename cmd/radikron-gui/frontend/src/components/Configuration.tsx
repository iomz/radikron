import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppStore } from '@/store/useAppStore';
import * as App from '../../wailsjs/go/main/App';
import { getAreaName } from '@/lib/regions';

export const Configuration: React.FC = () => {
  const configInfo = useAppStore((state) => state.configInfo);
  const addActivityLog = useAppStore((state) => state.addActivityLog);

  const handleOpenDirectory = async (dirPath: string) => {
    if (!dirPath || dirPath === 'N/A') {
      return;
    }
    try {
      await App.OpenDirectory(dirPath);
    } catch (error) {
      console.error('Failed to open directory:', error);
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      addActivityLog('error', `Failed to open directory: ${errorMessage}`);
    }
  };

  return (
    <Card className="md:col-span-2 lg:col-span-1">
      <CardHeader>
        <CardTitle>Configuration</CardTitle>
        <CardDescription>Current configuration settings</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {configInfo ? (
          <div className="space-y-1 text-sm">
            <p>
              <span className="font-medium">Area ID:</span>{' '}
              {configInfo.AreaID ? (
                <>
                  {configInfo.AreaID}
                  {getAreaName(configInfo.AreaID) && (
                    <span className="text-muted-foreground ml-2">
                      ({getAreaName(configInfo.AreaID)})
                    </span>
                  )}
                </>
              ) : (
                'N/A'
              )}
            </p>
            <p>
              <span className="font-medium">File Format:</span> {configInfo.FileFormat || 'N/A'}
            </p>
            <p>
              <span className="font-medium">Download Dir:</span>{' '}
              {configInfo.DownloadDir && configInfo.DownloadDir !== 'N/A' ? (
                <button
                  onClick={() => handleOpenDirectory(configInfo.DownloadDir)}
                  className="text-primary hover:underline cursor-pointer"
                  type="button"
                >
                  {configInfo.DownloadDir}
                </button>
              ) : (
                'N/A'
              )}
            </p>
            <p>
              <span className="font-medium">Rules:</span> {configInfo.Rules ? configInfo.Rules.length : 0}
            </p>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No configuration loaded</p>
        )}
      </CardContent>
    </Card>
  );
};

