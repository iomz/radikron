import React from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { useAppStore } from '@/store/useAppStore';

export const Configuration: React.FC = () => {
  const configInfo = useAppStore((state) => state.configInfo);
  const configFile = useAppStore((state) => state.configFile);
  const setConfigFile = useAppStore((state) => state.setConfigFile);
  const loadConfig = useAppStore((state) => state.loadConfig);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const refreshStations = useAppStore((state) => state.refreshStations);

  const handleLoadConfig = async () => {
    // Load the configuration file
    await loadConfig(configFile);
    // Refresh the displayed config info and stations to reflect the newly loaded configuration
    // Note: loadConfig catches errors internally, so we refresh regardless to update the UI
    try {
      await Promise.all([loadConfigInfo(), refreshStations()]);
    } catch (error) {
      // Refresh methods handle errors internally, but we log here for debugging
      console.error('Failed to refresh config info after load:', error);
    }
  };

  return (
    <Card className="md:col-span-2 lg:col-span-1">
      <CardHeader>
        <CardTitle>Configuration</CardTitle>
        <CardDescription>Load and manage configuration files</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="config-file">Config File</Label>
          <div className="flex gap-2">
            <Input
              id="config-file"
              value={configFile}
              onChange={(e) => setConfigFile(e.target.value)}
              placeholder="config.yml"
            />
            <Button variant="outline" onClick={handleLoadConfig}>
              Load
            </Button>
          </div>
        </div>
        {configInfo && (
          <div className="space-y-2 pt-2">
            <Separator />
            <div className="space-y-1 text-sm">
              <p>
                <span className="font-medium">Area ID:</span> {configInfo.AreaID || 'N/A'}
              </p>
              <p>
                <span className="font-medium">File Format:</span> {configInfo.FileFormat || 'N/A'}
              </p>
              <p>
                <span className="font-medium">Download Dir:</span> {configInfo.DownloadDir || 'N/A'}
              </p>
              <p>
                <span className="font-medium">Rules:</span> {configInfo.Rules ? configInfo.Rules.length : 0}
              </p>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
};

