/**
 * Targeted model availability mutations.
 *
 * The Models management page toggles single models on/off without rewriting a
 * whole provider section, so each mutation here is a small PATCH request
 * against the backend config endpoints (which support `excluded-models` /
 * `models` patches keyed by `match` or `index`).
 */

import { apiClient } from './client';
import type { ProviderBrand } from '@/features/providers/types';
import type { ModelAlias } from '@/types';

/** Config key sections that expose an `excluded-models` PATCH field. */
export type ConfigKeyBrand =
  | 'gemini'
  | 'interactions'
  | 'codex'
  | 'meta'
  | 'xai'
  | 'claude'
  | 'vertex';

const KEY_BRAND_ENDPOINTS: Record<ConfigKeyBrand, string> = {
  gemini: '/gemini-api-key',
  interactions: '/interactions-api-key',
  codex: '/codex-api-key',
  meta: '/meta-api-key',
  xai: '/xai-api-key',
  claude: '/claude-api-key',
  vertex: '/vertex-api-key',
};

export const isConfigKeyBrand = (brand: ProviderBrand): brand is ConfigKeyBrand =>
  Object.prototype.hasOwnProperty.call(KEY_BRAND_ENDPOINTS, brand);

/**
 * Updates `excluded-models` on one config key entry.
 *
 * When an API key is available the entry is targeted by `match` (identity
 * based) so a re-indexed config cannot cause the wrong entry to be patched.
 * Keyless entries fall back to the section index.
 */
export async function patchConfigKeyExcludedModels(args: {
  brand: ConfigKeyBrand;
  index: number;
  apiKey?: string | null;
  excludedModels: string[];
}): Promise<void> {
  const endpoint = KEY_BRAND_ENDPOINTS[args.brand];
  const value = { 'excluded-models': args.excludedModels };
  const apiKey = args.apiKey?.trim();
  const body = apiKey
    ? { match: apiKey, value }
    : { index: args.index, value };
  await apiClient.patch(endpoint, body);
}

const serializeOpenAICompatModel = (model: ModelAlias): Record<string, unknown> => {
  const payload: Record<string, unknown> = { name: model.name };
  if (model.alias && model.alias !== model.name) {
    payload.alias = model.alias;
  }
  if (model.priority !== undefined) {
    payload.priority = model.priority;
  }
  if (model.testModel) {
    payload['test-model'] = model.testModel;
  }
  if (model.image) {
    payload.image = true;
  }
  if (model.thinking) {
    payload.thinking = model.thinking;
  }
  return payload;
};

/** Replaces the `models` list of one OpenAI compatibility provider. */
export async function patchOpenAICompatModels(args: {
  name: string;
  index: number;
  models: ModelAlias[];
}): Promise<void> {
  const serialized = args.models
    .map(serializeOpenAICompatModel)
    .filter((model) => model.name);
  const payload = serialized.length ? { models: serialized } : { models: [] };
  const name = args.name.trim();
  const body = name
    ? { name, value: payload }
    : { index: args.index, value: payload };
  await apiClient.patch('/openai-compatibility', body);
}

/** Updates per-auth-file excluded models via the auth files field endpoint. */
export async function patchAuthFileExcludedModels(args: {
  name: string;
  excludeFieldKey: 'excluded_models' | 'excluded-models';
  excludedModels: string[];
}): Promise<void> {
  await apiClient.patch('/auth-files/fields', {
    name: args.name,
    [args.excludeFieldKey]: args.excludedModels,
  });
}
