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

  const [isLoading, setIsLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const handleLoadConfig = async () => {
    setIsLoading(true);
    setError(null);
    try {
      // Load the configuration file
      await loadConfig(configFile);
      // Small delay to allow backend to process the loaded config
      await new Promise(resolve => setTimeout(resolve, 100));
      // Refresh the displayed config info and stations to reflect the newly loaded configuration
      await Promise.all([loadConfigInfo(), refreshStations()]);
    } catch (error) {
      console.error('Failed to load configuration:', error);
      setError('Failed to load configuration. Please try again.');
    } finally {
      setIsLoading(false);
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
          <form onSubmit={(e) => { e.preventDefault(); if (configFile.trim() && !isLoading) handleLoadConfig(); }} className="flex gap-2">
            <Input
              id="config-file"
              value={configFile}
              onChange={(e) => setConfigFile(e.target.value)}
              placeholder="config.yml"
              required
              disabled={isLoading}
            />
            <Button type="submit" variant="outline" disabled={!configFile.trim() || isLoading}>
              {isLoading ? 'Loading...' : 'Load'}
            </Button>
          </form>
          {error && <p className="text-sm text-red-500">{error}</p>}
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

