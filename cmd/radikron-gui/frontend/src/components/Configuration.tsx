import React, { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useAppStore } from '@/store/useAppStore';
import * as App from '../../wailsjs/go/main/App';
import { config } from '../../wailsjs/go/models';
import { getAreaName, regionsData } from '@/lib/regions';
import { toast } from 'sonner';
import { X, Plus } from 'lucide-react';

// Flatten all regions into a single array for the select dropdown
const allRegions = Object.values(regionsData).flat();

export const Configuration: React.FC = () => {
  const configInfo = useAppStore((state) => state.configInfo);
  const stations = useAppStore((state) => state.stations);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [allStations, setAllStations] = useState<string[]>([]);
  const [editedConfig, setEditedConfig] = useState<{
    AreaID: string;
    FileFormat: string;
    DownloadDir: string;
    ExtraStations: string[];
    IgnoreStations: string[];
    MinimumOutputSize: string; // In MB, as string for input
    MaxDownloadingConcurrency: string; // As string for input
    MaxEncodingConcurrency: string; // As string for input
  } | null>(null);
  const [selectedExtraStation, setSelectedExtraStation] = useState<string>('');
  const [selectedIgnoreStation, setSelectedIgnoreStation] = useState<string>('');

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

  const handleExport = async () => {
    try {
      await App.ExportConfigFile();
      toast.success('Configuration exported successfully');
      addActivityLog('success', 'Configuration exported');
    } catch (error) {
      console.error('Failed to export config:', error);
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      // Don't show error if user cancelled the dialog
      if (!errorMessage.includes('cancelled')) {
        toast.error(`Failed to export config: ${errorMessage}`);
        addActivityLog('error', `Failed to export config: ${errorMessage}`);
      }
    }
  };

  const handleEdit = async () => {
    if (!configInfo) return;
    // Convert MinimumOutputSize from bytes to MB (1 MB = 1024 * 1024 bytes)
    const minimumOutputSizeMB = configInfo.MinimumOutputSize != null
      ? (configInfo.MinimumOutputSize / (1024 * 1024)).toString()
      : '1';
    
    // Load all stations for Extra Stations selector
    try {
      const allStationsList = await App.GetAllStations();
      setAllStations(allStationsList);
    } catch (error) {
      console.error('Failed to load all stations:', error);
      setAllStations([]);
    }
    
    setEditedConfig({
      AreaID: configInfo.AreaID || '',
      FileFormat: configInfo.FileFormat || 'aac',
      DownloadDir: configInfo.DownloadDir || '',
      ExtraStations: configInfo.ExtraStations || [],
      IgnoreStations: configInfo.IgnoreStations || [],
      MinimumOutputSize: minimumOutputSizeMB,
      MaxDownloadingConcurrency: configInfo.MaxDownloadingConcurrency?.toString() || '64',
      MaxEncodingConcurrency: configInfo.MaxEncodingConcurrency?.toString() || '2',
    });
    setSelectedExtraStation('');
    setSelectedIgnoreStation('');
    setIsEditDialogOpen(true);
  };

  const handleSave = async () => {
    if (!editedConfig || !configInfo) return;

    try {
      // Use the station arrays directly
      const extraStations = editedConfig.ExtraStations || [];
      const ignoreStations = editedConfig.IgnoreStations || [];

      // Convert MinimumOutputSize from MB to bytes
      const minimumOutputSizeMB = parseFloat(editedConfig.MinimumOutputSize) || 1;
      const minimumOutputSizeBytes = Math.round(minimumOutputSizeMB * 1024 * 1024);

      // Parse numeric fields
      const maxDownloadingConcurrency = parseInt(editedConfig.MaxDownloadingConcurrency, 10) || 64;
      const maxEncodingConcurrency = parseInt(editedConfig.MaxEncodingConcurrency, 10) || 2;

      // Create updated config object with all required fields
      const updatedConfig = new config.Config({
        ...configInfo,
        AreaID: editedConfig.AreaID,
        FileFormat: editedConfig.FileFormat,
        DownloadDir: editedConfig.DownloadDir,
        ExtraStations: extraStations,
        IgnoreStations: ignoreStations,
        MinimumOutputSize: minimumOutputSizeBytes,
        MaxDownloadingConcurrency: maxDownloadingConcurrency,
        MaxEncodingConcurrency: maxEncodingConcurrency,
      });

      await App.UpdateConfig(updatedConfig);
      await App.SaveConfig('');
      await loadConfigInfo();

      setIsEditDialogOpen(false);
      toast.success('Configuration saved successfully');
      addActivityLog('success', 'Configuration updated');
    } catch (error) {
      console.error('Failed to save config:', error);
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      toast.error(`Failed to save config: ${errorMessage}`);
      addActivityLog('error', `Failed to save config: ${errorMessage}`);
    }
  };

  const handleCancel = () => {
    setIsEditDialogOpen(false);
    setEditedConfig(null);
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
        <div className="flex gap-2 pt-2">
          <Button
            onClick={handleExport}
            disabled={!configInfo}
            className="flex-1"
            variant="outline"
          >
            Export
          </Button>
          <Button
            onClick={handleEdit}
            disabled={!configInfo}
            className="flex-1"
            variant="outline"
          >
            Edit
          </Button>
        </div>
      </CardContent>

      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent className="text-foreground [&>button]:text-foreground [&>button:hover]:text-foreground [&>button]:opacity-100 [&>button>svg]:text-foreground max-h-[90vh] overflow-y-auto max-w-4xl">
          <DialogHeader>
            <DialogTitle className="text-foreground">Edit Configuration</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Edit the configuration parameters below.
            </DialogDescription>
          </DialogHeader>
          {editedConfig && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="area-id" className="text-foreground">Area ID</Label>
                <Select
                  value={editedConfig.AreaID}
                  onValueChange={(value) =>
                    setEditedConfig({ ...editedConfig, AreaID: value })
                  }
                >
                  <SelectTrigger id="area-id" className="text-foreground">
                    <SelectValue placeholder="Select area" />
                  </SelectTrigger>
                  <SelectContent className="text-foreground">
                    {allRegions.map((region) => (
                      <SelectItem key={region.id} value={region.id}>
                        {region.id} - {region.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="file-format" className="text-foreground">File Format</Label>
                <Select
                  value={editedConfig.FileFormat}
                  onValueChange={(value) =>
                    setEditedConfig({
                      ...editedConfig,
                      FileFormat: value,
                    })
                  }
                >
                  <SelectTrigger id="file-format" className="text-foreground">
                    <SelectValue placeholder="Select format" />
                  </SelectTrigger>
                  <SelectContent className="text-foreground">
                    <SelectItem value="aac">AAC</SelectItem>
                    <SelectItem value="mp3">MP3</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2 md:col-span-2">
                <Label htmlFor="download-dir" className="text-foreground">Download Directory</Label>
                <div className="flex gap-2">
                  <Input
                    id="download-dir"
                    value={editedConfig.DownloadDir}
                    readOnly
                    placeholder="/path/to/downloads"
                    className="text-foreground flex-1"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={async () => {
                      try {
                        const selectedPath = await App.SelectDirectory();
                        setEditedConfig({
                          ...editedConfig,
                          DownloadDir: selectedPath,
                        });
                      } catch (error) {
                        // Don't show error if user cancelled the dialog
                        const errorMessage =
                          error instanceof Error ? error.message : String(error);
                        if (!errorMessage.includes('cancelled')) {
                          toast.error(`Failed to select directory: ${errorMessage}`);
                        }
                      }
                    }}
                  >
                    Browse
                  </Button>
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="extra-stations" className="text-foreground">
                  Extra Stations
                </Label>
                <div className="flex gap-2">
                  <Select
                    value={selectedExtraStation}
                    onValueChange={setSelectedExtraStation}
                  >
                    <SelectTrigger className="text-foreground flex-1">
                      <SelectValue placeholder="Select station" />
                    </SelectTrigger>
                    <SelectContent className="text-foreground">
                      {allStations
                        .filter((station) => 
                          !editedConfig.ExtraStations.includes(station) &&
                          !stations.includes(station)
                        )
                        .map((station) => (
                          <SelectItem key={station} value={station}>
                            {station}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => {
                      if (selectedExtraStation && !editedConfig.ExtraStations.includes(selectedExtraStation)) {
                        setEditedConfig({
                          ...editedConfig,
                          ExtraStations: [...editedConfig.ExtraStations, selectedExtraStation],
                        });
                        setSelectedExtraStation('');
                      }
                    }}
                    disabled={!selectedExtraStation}
                  >
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
                {editedConfig.ExtraStations.length > 0 && (
                  <div className="flex flex-wrap gap-2 mt-2">
                    {editedConfig.ExtraStations.map((station) => (
                      <Badge
                        key={station}
                        variant="secondary"
                        className="text-foreground"
                      >
                        {station}
                        <button
                          type="button"
                          onClick={() => {
                            setEditedConfig({
                              ...editedConfig,
                              ExtraStations: editedConfig.ExtraStations.filter((s) => s !== station),
                            });
                          }}
                          className="ml-2 hover:bg-destructive/20 rounded-full p-0.5"
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </Badge>
                    ))}
                  </div>
                )}
                <p className="text-xs text-muted-foreground">
                  Station IDs to include even if they're not in your region
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="ignore-stations" className="text-foreground">
                  Ignore Stations
                </Label>
                <div className="flex gap-2">
                  <Select
                    value={selectedIgnoreStation}
                    onValueChange={setSelectedIgnoreStation}
                  >
                    <SelectTrigger className="text-foreground flex-1">
                      <SelectValue placeholder="Select station" />
                    </SelectTrigger>
                    <SelectContent className="text-foreground">
                      {stations
                        .filter((station) => !editedConfig.IgnoreStations.includes(station))
                        .map((station) => (
                          <SelectItem key={station} value={station}>
                            {station}
                          </SelectItem>
                        ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => {
                      if (selectedIgnoreStation && !editedConfig.IgnoreStations.includes(selectedIgnoreStation)) {
                        setEditedConfig({
                          ...editedConfig,
                          IgnoreStations: [...editedConfig.IgnoreStations, selectedIgnoreStation],
                        });
                        setSelectedIgnoreStation('');
                      }
                    }}
                    disabled={!selectedIgnoreStation}
                  >
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
                {editedConfig.IgnoreStations.length > 0 && (
                  <div className="flex flex-wrap gap-2 mt-2">
                    {editedConfig.IgnoreStations.map((station) => (
                      <Badge
                        key={station}
                        variant="secondary"
                        className="text-foreground"
                      >
                        {station}
                        <button
                          type="button"
                          onClick={() => {
                            setEditedConfig({
                              ...editedConfig,
                              IgnoreStations: editedConfig.IgnoreStations.filter((s) => s !== station),
                            });
                          }}
                          className="ml-2 hover:bg-destructive/20 rounded-full p-0.5"
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </Badge>
                    ))}
                  </div>
                )}
                <p className="text-xs text-muted-foreground">
                  Station IDs to exclude from monitoring
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="minimum-output-size" className="text-foreground">
                  Minimum Output Size (MB)
                </Label>
                <Input
                  id="minimum-output-size"
                  type="number"
                  min="0"
                  step="0.1"
                  value={editedConfig.MinimumOutputSize}
                  onChange={(e) =>
                    setEditedConfig({
                      ...editedConfig,
                      MinimumOutputSize: e.target.value,
                    })
                  }
                  placeholder="1"
                  className="text-foreground"
                />
                <p className="text-xs text-muted-foreground">
                  Minimum file size in MB. Files smaller than this are rejected as potentially corrupted (default: 1 MB)
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="max-downloading-concurrency" className="text-foreground">
                  Max Downloading Concurrency
                </Label>
                <Input
                  id="max-downloading-concurrency"
                  type="number"
                  min="1"
                  value={editedConfig.MaxDownloadingConcurrency}
                  onChange={(e) =>
                    setEditedConfig({
                      ...editedConfig,
                      MaxDownloadingConcurrency: e.target.value,
                    })
                  }
                  placeholder="64"
                  className="text-foreground"
                />
                <p className="text-xs text-muted-foreground">
                  Maximum number of concurrent download operations (default: 64)
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="max-encoding-concurrency" className="text-foreground">
                  Max Encoding Concurrency
                </Label>
                <Input
                  id="max-encoding-concurrency"
                  type="number"
                  min="1"
                  value={editedConfig.MaxEncodingConcurrency}
                  onChange={(e) =>
                    setEditedConfig({
                      ...editedConfig,
                      MaxEncodingConcurrency: e.target.value,
                    })
                  }
                  placeholder="2"
                  className="text-foreground"
                />
                <p className="text-xs text-muted-foreground">
                  Maximum number of concurrent MP3 encoding operations (default: 2). Set lower than downloading concurrency since encoding is CPU-intensive.
                </p>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button onClick={handleCancel} variant="outline">
              Cancel
            </Button>
            <Button onClick={handleSave}>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
};

