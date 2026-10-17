/**
 * Upstream model discovery for the Models management page.
 *
 * Reuses the same management API proxy (`/api-call`) based fetchers as the
 * provider edit form, but driven directly from credential data instead of
 * form state so cards can fetch catalogs in bulk.
 */

import { modelsApi } from '@/services/api';
import { getErrorMessage } from '@/utils/helpers';
import type { ModelInfo } from '@/utils/models';
import type { ProviderBrand } from '@/features/providers/types';

export interface UpstreamDiscoveryInput {
  /** Brand determines the discovery endpoint shape. */
  brand: ProviderBrand;
  baseUrl?: string | null;
  apiKey?: string | null;
  authIndex?: string | null;
  headers?: Record<string, string>;
  apiKeyEntries?: Array<{ apiKey?: string; existingApiKey?: string; authIndex?: string }>;
}

const SUPPORTED_DISCOVERY_BRANDS: ReadonlyArray<ProviderBrand> = [
  'gemini',
  'interactions',
  'codex',
  'meta',
  'xai',
  'claude',
  'openaiCompatibility',
];

export const isUpstreamDiscoveryBrand = (brand: ProviderBrand): boolean =>
  SUPPORTED_DISCOVERY_BRANDS.includes(brand);

type ApiKeyEntryLike = NonNullable<UpstreamDiscoveryInput['apiKeyEntries']>[number];

const firstUsableEntry = (
  entries?: ApiKeyEntryLike[]
): ApiKeyEntryLike | undefined => {
  return (entries ?? []).find(
    (entry) =>
      (entry.apiKey ?? '').trim() ||
      (entry.existingApiKey ?? '').trim() ||
      (entry.authIndex ?? '').trim()
  );
};

export async function fetchUpstreamModels(input: UpstreamDiscoveryInput): Promise<ModelInfo[]> {
  const baseUrl = input.baseUrl ?? '';
  const headers = input.headers && Object.keys(input.headers).length ? input.headers : {};
  const authIndex = input.authIndex?.trim() || undefined;

  if (input.brand === 'openaiCompatibility') {
    const entry = firstUsableEntry(input.apiKeyEntries);
    const entryKey =
      (entry?.apiKey ?? '').trim() || (entry?.existingApiKey ?? '').trim();
    const entryAuthIndex = (entry?.authIndex ?? '').trim() || authIndex;
    try {
      return await modelsApi.fetchModelsViaApiCall(baseUrl, entryKey, headers, entryAuthIndex);
    } catch (firstErr) {
      // Some OpenAI-compatible endpoints expose /models without auth, or
      // reject the configured key for the discovery route. Retry once
      // without any auth/headers before surfacing the original error.
      try {
        return await modelsApi.fetchModelsViaApiCall(baseUrl);
      } catch {
        throw firstErr;
      }
    }
  }

  if (
    input.brand === 'gemini' ||
    input.brand === 'interactions'
  ) {
    return modelsApi.fetchGeminiModelsViaApiCall(baseUrl, input.apiKey ?? '', headers, authIndex);
  }

  if (input.brand === 'codex' || input.brand === 'meta' || input.brand === 'xai') {
    return modelsApi.fetchV1ModelsViaApiCall(baseUrl, input.apiKey ?? '', headers, authIndex);
  }

  if (input.brand === 'claude') {
    return modelsApi.fetchClaudeModelsViaApiCall(baseUrl, input.apiKey ?? '', headers, authIndex);
  }

  throw new Error(`Model discovery is not available for ${input.brand}`);
}

export const getDiscoveryErrorMessage = (err: unknown): string =>
  getErrorMessage(err) || 'Failed to fetch models';
