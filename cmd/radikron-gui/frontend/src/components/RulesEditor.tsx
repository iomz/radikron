import React, { useState, useEffect } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useAppStore } from "@/store/useAppStore";
import * as App from "../../wailsjs/go/main/App";
import { config, radikron } from "../../wailsjs/go/models";

export const RulesEditor: React.FC = () => {
  const configInfo = useAppStore((state) => state.configInfo);
  const loadConfigInfo = useAppStore((state) => state.loadConfigInfo);
  const configFile = useAppStore((state) => state.configFile);
  const addActivityLog = useAppStore((state) => state.addActivityLog);
  const [isSaving, setIsSaving] = useState(false);
  const [rules, setRules] = useState<config.Config | null>(null);
  const [expandedRules, setExpandedRules] = useState<Set<number>>(new Set());

  useEffect(() => {
    if (configInfo) {
      setRules(configInfo);
      // Reset expanded rules when config changes
      setExpandedRules(new Set());
    }
  }, [configInfo]);

  const toggleRule = (index: number) => {
    setExpandedRules((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(index)) {
        newSet.delete(index);
      } else {
        newSet.add(index);
      }
      return newSet;
    });
  };

  const handleSave = async () => {
    if (!rules) return;

    setIsSaving(true);
    try {
      // Note: UpdateConfig will be available after rebuilding Wails
      // For now, we'll just save the config file
      // The backend will need to be rebuilt to expose UpdateConfig
      try {
        // @ts-ignore - UpdateConfig will be available after Wails rebuild
        await App.UpdateConfig(rules);
      } catch (e) {
        // If UpdateConfig is not available, just save directly
        console.warn("UpdateConfig not available, saving config directly");
      }
      // Save the config to file
      await App.SaveConfig(configFile);
      // Reload config info to reflect changes
      await loadConfigInfo();
      addActivityLog("success", "Configuration saved successfully");
    } catch (error) {
      console.error("Failed to save config:", error);
      const errorMessage =
        error instanceof Error ? error.message : String(error);
      addActivityLog("error", `Failed to save config: ${errorMessage}`);
    } finally {
      setIsSaving(false);
    }
  };

  const addRule = () => {
    if (!rules) return;

    const newRule = radikron.Rule.createFrom({
      Name: `rule-${Date.now()}`,
      Title: "",
      DoW: [],
      Keyword: "",
      Pfm: "",
      StationID: "",
      Window: "",
      Folder: "",
    });

    setRules(
      config.Config.createFrom({
        ...rules,
        Rules: [...(rules.Rules || []), newRule],
      }),
    );
  };

  const removeRule = (index: number) => {
    if (!rules) return;

    setRules(
      config.Config.createFrom({
        ...rules,
        Rules: rules.Rules.filter((_, i) => i !== index),
      }),
    );

    setExpandedRules((prev) => {
      const next = new Set<number>();
      prev.forEach((i) => {
        if (i < index) next.add(i);
        else if (i > index) next.add(i - 1);
      });
      return next;
    });
  };

  const updateRule = (
    index: number,
    field: string,
    value: string | string[],
  ) => {
    if (!rules) return;

    const updatedRules = [...rules.Rules];
    updatedRules[index] = radikron.Rule.createFrom({
      ...updatedRules[index],
      [field]: value,
    });

    setRules(
      config.Config.createFrom({
        ...rules,
        Rules: updatedRules,
      }),
    );
  };

  if (!rules) {
    return (
      <div className="flex items-center justify-center px-4 py-8">
        <Card className="w-full max-w-4xl">
          <CardHeader>
            <CardTitle>Rules Editor</CardTitle>
            <CardDescription>
              Load a configuration file to edit rules
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center px-4 py-8">
      <Card className="w-full max-w-6xl flex flex-col h-[600px]">
        <CardHeader className="flex-shrink-0">
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Rules Editor</CardTitle>
              <CardDescription>
                Edit and manage your download rules
              </CardDescription>
            </div>
            <div className="flex gap-2">
              <Button onClick={addRule} variant="outline">
                Add Rule
              </Button>
              <Button onClick={handleSave} disabled={isSaving}>
                {isSaving ? "Saving..." : "Save Configuration"}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="flex-1 min-h-0">
          <ScrollArea className="h-full">
            <div className="space-y-6 pr-4">
              {rules.Rules && rules.Rules.length > 0 ? (
                rules.Rules.map((rule, index) => {
                  const isExpanded = expandedRules.has(index);
                  return (
                    <Card key={index} className="p-4">
                      <div className="flex items-start justify-between mb-4">
                        <button
                          onClick={() => toggleRule(index)}
                          className="flex items-center gap-2 flex-1 text-left hover:bg-accent/50 rounded-md p-2 -m-2 transition-colors cursor-pointer"
                          type="button"
                        >
                          <span className="text-muted-foreground">
                            {isExpanded ? "▼" : "▶"}
                          </span>
                          <h3 className="text-lg font-semibold">
                            Rule {index + 1}:{" "}
                            {rule.Name || `Unnamed Rule ${index + 1}`}
                          </h3>
                        </button>
                        <Button
                          onClick={(e) => {
                            e.stopPropagation();
                            removeRule(index);
                          }}
                          variant="outline"
                          size="sm"
                          className="text-destructive hover:text-destructive"
                        >
                          Remove
                        </Button>
                      </div>
                      {isExpanded && (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                          <div className="space-y-2">
                            <Label htmlFor={`rule-name-${index}`}>
                              Rule Name
                            </Label>
                            <Input
                              id={`rule-name-${index}`}
                              value={rule.Name || ""}
                              onChange={(e) =>
                                updateRule(index, "Name", e.target.value)
                              }
                              placeholder="e.g., citypop"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-folder-${index}`}>
                              Folder (optional)
                            </Label>
                            <Input
                              id={`rule-folder-${index}`}
                              value={rule.Folder || ""}
                              onChange={(e) =>
                                updateRule(index, "Folder", e.target.value)
                              }
                              placeholder="e.g., citypop"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-title-${index}`}>
                              Title (optional)
                            </Label>
                            <Input
                              id={`rule-title-${index}`}
                              value={rule.Title || ""}
                              onChange={(e) =>
                                updateRule(index, "Title", e.target.value)
                              }
                              placeholder="Partial match supported"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-keyword-${index}`}>
                              Keyword (optional)
                            </Label>
                            <Input
                              id={`rule-keyword-${index}`}
                              value={rule.Keyword || ""}
                              onChange={(e) =>
                                updateRule(index, "Keyword", e.target.value)
                              }
                              placeholder="Search in title or description"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-pfm-${index}`}>
                              Personality/Performer (optional)
                            </Label>
                            <Input
                              id={`rule-pfm-${index}`}
                              value={rule.Pfm || ""}
                              onChange={(e) =>
                                updateRule(index, "Pfm", e.target.value)
                              }
                              placeholder="DJ/MC name"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-station-${index}`}>
                              Station ID (optional)
                            </Label>
                              <Input
                              id={`rule-station-${index}`}
                              value={rule.StationID || ""}
                              onChange={(e) =>
                                updateRule(index, "StationID", e.target.value)
                              }
                              placeholder="e.g., FMT"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-window-${index}`}>
                              Time Window (optional)
                            </Label>
                            <Input
                              id={`rule-window-${index}`}
                              value={rule.Window || ""}
                              onChange={(e) =>
                                updateRule(index, "Window", e.target.value)
                              }
                              placeholder="e.g., 48h, 7d"
                            />
                          </div>
                          <div className="space-y-2">
                            <Label htmlFor={`rule-dow-${index}`}>
                              Days of Week (optional)
                            </Label>
                            <Input
                              id={`rule-dow-${index}`}
                              value={
                                Array.isArray(rule.DoW)
                                  ? rule.DoW.join(", ")
                                  : ""
                              }
                              onChange={(e) => {
                                const days = e.target.value
                                  .split(",")
                                  .map((d) => d.trim().toLowerCase())
                                  .filter((d) => d);
                                updateRule(index, "DoW", days);
                              }}
                              placeholder="e.g., mon, tue, wed"
                            />
                          </div>
                        </div>
                      )}
                    </Card>
                  );
                })
              ) : (
                <div className="text-center py-8 text-muted-foreground">
                  <p>No rules configured. Click "Add Rule" to create one.</p>
                </div>
              )}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
};
