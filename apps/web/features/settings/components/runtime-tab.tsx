'use client';

import { useState } from 'react';
import { api } from '@/shared/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface CloudRuntimeConfig {
  id: string;
  provider: 'anthropic' | 'openai';
  gateway_url: string | null;
  is_active: boolean;
  max_concurrent_tasks: number;
}

export function RuntimeTab() {
  const [runtimeType, setRuntimeType] = useState<'local' | 'cloud'>('local');
  const [provider, setProvider] = useState<'anthropic' | 'openai'>('anthropic');
  const [apiKey, setApiKey] = useState('');
  const [gatewayUrl, setGatewayUrl] = useState('');
  const [isValidating, setIsValidating] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const handleTestConnection = async () => {
    setIsValidating(true);
    setError(null);
    setSuccess(false);

    try {
      await api.validateCloudRuntime(provider, apiKey);
      setSuccess(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Connection test failed");
    } finally {
      setIsValidating(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);

    try {
      await api.saveCloudRuntime({
        provider,
        api_key: apiKey,
        gateway_url: runtimeType === "cloud" && gatewayUrl ? gatewayUrl : null,
        max_concurrent_tasks: 3,
      });

      setSuccess(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Runtime Settings</h2>
        <p className="text-sm text-muted-foreground">
          Configure where agents execute tasks
        </p>
      </div>

      {/* Runtime Type Selector */}
      <div className="space-y-2">
        <Label>Runtime Type</Label>
        <div className="flex gap-4">
          <button
            type="button"
            onClick={() => setRuntimeType('local')}
            className={`flex-1 p-4 border rounded-lg text-left ${
              runtimeType === 'local' ? 'border-primary bg-primary/5' : 'border-border'
            }`}
          >
            <div className="font-medium">Local Daemon</div>
            <div className="text-sm text-muted-foreground">
              Agents run on your machine
            </div>
          </button>
          <button
            type="button"
            onClick={() => setRuntimeType('cloud')}
            className={`flex-1 p-4 border rounded-lg text-left ${
              runtimeType === 'cloud' ? 'border-primary bg-primary/5' : 'border-border'
            }`}
          >
            <div className="font-medium">Cloud Runtime</div>
            <div className="text-sm text-muted-foreground">
              Agents run in managed containers
            </div>
          </button>
        </div>
      </div>

      {runtimeType === 'cloud' && (
        <>
          {/* Provider Selection */}
          <div className="space-y-2">
            <Label>AI Provider</Label>
            <Select value={provider} onValueChange={(v) => setProvider(v as typeof provider)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="anthropic">Anthropic (Claude)</SelectItem>
                <SelectItem value="openai">OpenAI (GPT)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* API Key */}
          <div className="space-y-2">
            <Label>API Key</Label>
            <div className="flex gap-2">
              <Input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-ant-..."
                className="flex-1"
              />
              <Button
                variant="outline"
                onClick={handleTestConnection}
                disabled={!apiKey || isValidating}
              >
                {isValidating ? 'Testing...' : 'Test'}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Your API key is encrypted and never exposed
            </p>
          </div>

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}

          {success && (
            <p className="text-sm text-green-600">Settings saved successfully</p>
          )}

          {/* Save Button */}
          <Button onClick={handleSave} disabled={isSaving || !apiKey}>
            {isSaving ? 'Saving...' : 'Save Cloud Runtime'}
          </Button>
        </>
      )}
    </div>
  );
}
