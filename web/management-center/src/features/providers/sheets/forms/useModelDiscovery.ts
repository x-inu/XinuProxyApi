import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { modelsApi } from '@/services/api';
import { buildHeaderObject } from '@/utils/headers';
import { getErrorMessage } from '@/utils/helpers';
import type { ModelInfo } from '@/utils/models';
import type { ApiKeyEntryInput, ProviderBrand } from '../../types';

export const MODEL_DISCOVERY_BRANDS: ReadonlyArray<ProviderBrand> = [
  'gemini',
  'interactions',
  'codex',
  'meta',
  'xai',
  'claude',
  'openaiCompatibility',
];

export const isModelDiscoveryBrand = (brand: ProviderBrand): boolean =>
  MODEL_DISCOVERY_BRANDS.includes(brand);

export interface UseModelDiscoveryArgs {
  brand: ProviderBrand;
  baseUrl: string;
  formHeaders: Array<{ key: string; value: string }>;
  apiKeyEntries?: ApiKeyEntryInput[];
  apiKey?: string;
  fallbackApiKey?: string;
  authIndex?: string;
}

export interface UseModelDiscoveryResult {
  available: boolean;
  brand: ProviderBrand;
  loading: boolean;
  error: string | null;
  models: ModelInfo[];
  hasFetched: boolean;
  freeOnly: boolean;
  setFreeOnly: (value: boolean) => void;
  fetch: () => Promise<void>;
  reset: () => void;
}

export function useModelDiscovery(args: UseModelDiscoveryArgs): UseModelDiscoveryResult {
  const { brand, baseUrl, formHeaders, apiKeyEntries, apiKey, fallbackApiKey, authIndex } = args;

  const available = isModelDiscoveryBrand(brand);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [rawModels, setRawModels] = useState<ModelInfo[]>([]);
  const [hasFetched, setHasFetched] = useState(false);
  const [freeOnly, setFreeOnly] = useState(false);

  const fetch = useCallback(async () => {
    if (!available) return;
    setLoading(true);
    setError(null);
    try {
      const baseHeaders = buildHeaderObject(formHeaders);
      const resolvedAuthIndex = (authIndex ?? '').trim() || undefined;
      let next: ModelInfo[] = [];
      if (brand === 'gemini' || brand === 'interactions') {
        const key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        next = await modelsApi.fetchGeminiModelsViaApiCall(
          baseUrl,
          key,
          baseHeaders,
          resolvedAuthIndex
        );
      } else if (brand === 'codex' || brand === 'meta' || brand === 'xai') {
        const key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        next = await modelsApi.fetchV1ModelsViaApiCall(
          baseUrl,
          key,
          baseHeaders,
          resolvedAuthIndex
        );
      } else if (brand === 'claude') {
        const key = (apiKey ?? '').trim() || (fallbackApiKey ?? '').trim();
        next = await modelsApi.fetchClaudeModelsViaApiCall(
          baseUrl,
          key,
          baseHeaders,
          resolvedAuthIndex
        );
      } else if (brand === 'openaiCompatibility') {
        const firstEntry = (apiKeyEntries ?? []).find(
          (e) =>
            (e.apiKey ?? '').trim() || (e.existingApiKey ?? '').trim() || (e.authIndex ?? '').trim()
        );
        const entryKey =
          (firstEntry?.apiKey ?? '').trim() || (firstEntry?.existingApiKey ?? '').trim();
        const entryAuthIndex = (firstEntry?.authIndex ?? '').trim() || resolvedAuthIndex;
        try {
          next = await modelsApi.fetchModelsViaApiCall(
            baseUrl,
            entryKey,
            baseHeaders,
            entryAuthIndex
          );
        } catch (firstErr) {
          // Some OpenAI-compatible endpoints expose /models without auth, or
          // reject the configured key for the discovery route. Retry once
          // without any auth/headers before surfacing the original error.
          try {
            next = await modelsApi.fetchModelsViaApiCall(baseUrl);
          } catch {
            throw firstErr;
          }
        }
      }
      setRawModels(next ?? []);
      setHasFetched(true);
    } catch (err) {
      setRawModels([]);
      setError(getErrorMessage(err) || 'Failed to fetch models');
      setHasFetched(true);
    } finally {
      setLoading(false);
    }
  }, [available, apiKey, apiKeyEntries, authIndex, baseUrl, brand, fallbackApiKey, formHeaders]);

  const models = useMemo(() => {
    if (!freeOnly) return rawModels;
    return rawModels.filter((m) => m.isFree === true);
  }, [rawModels, freeOnly]);

  const reset = useCallback(() => {
    setRawModels([]);
    setError(null);
    setLoading(false);
    setHasFetched(false);
  }, []);

  const inputSignature = useMemo(() => {
    const headerSig = formHeaders.map((h) => `${h.key}:${h.value}`).join('|');
    const entriesSig = (apiKeyEntries ?? [])
      .map((e) => `${e.apiKey ?? ''}::${e.existingApiKey ?? ''}::${e.authIndex ?? ''}`)
      .join('|');
    return [
      baseUrl,
      apiKey ?? '',
      fallbackApiKey ?? '',
      authIndex ?? '',
      headerSig,
      entriesSig,
    ].join('||');
  }, [apiKey, apiKeyEntries, authIndex, baseUrl, fallbackApiKey, formHeaders]);

  const lastSignatureRef = useRef(inputSignature);
  useEffect(() => {
    if (lastSignatureRef.current === inputSignature) return;
    lastSignatureRef.current = inputSignature;
    reset();
  }, [inputSignature, reset]);

  return { available, brand, loading, error, models, hasFetched, freeOnly, setFreeOnly, fetch, reset };
}
