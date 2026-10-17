/**
 * Pure catalog logic for the Models management page: grouping provider
 * credentials, merging configured/discovered/static catalogs, and deriving
 * per-model availability rows. Kept framework-free so it is easy to test.
 */

import {
  DISABLE_ALL_RULE,
  hasExcludedRule,
  isMatchedByWildcardRule,
  isWildcardRule,
  matchesExcludedRule,
  normalizeExcludedRules,
  toggleExcludedRule,
} from '@/components/excludedModels/excludedModelRules';
import type { ProviderBrand } from '@/features/providers/types';
import type {
  Config,
  GeminiKeyConfig,
  ModelAlias,
  OpenAIProviderConfig,
  ProviderKeyConfig,
} from '@/types';
import { maskApiKey } from '@/utils/format';
import { isOpencodeFreeOpenAIProvider } from '../providers/opencodeFree';
import { isFennoAIClaudeProvider, isFennoAICodexProvider } from '../providers/fennoAI';
import {
  isQiniuCloudClaudeProvider,
  isQiniuCloudCodexProvider,
  isQiniuCloudGeminiProvider,
  isQiniuCloudOpenAIProvider,
} from '../providers/qiniuCloud';
import { isKimiClaudeProvider, isKimiCodexProvider, isKimiOpenAIProvider } from '../providers/kimi';
import type { ConfigKeyBrand } from '@/services/api/modelAvailability';
import { isUpstreamDiscoveryBrand, type UpstreamDiscoveryInput } from './upstreamDiscovery';

export type CredentialKind = 'config-key' | 'openai-compat' | 'oauth-file' | 'oauth-group';

export type CredentialMutation =
  | { type: 'config-key'; keyBrand: ConfigKeyBrand; index: number; apiKey: string | null }
  | { type: 'openai-compat'; name: string; index: number; models: ModelAlias[] }
  | { type: 'oauth-file'; fileName: string }
  /** Merged OAuth group card: every toggle writes the global rules layer. */
  | { type: 'oauth-group'; providerKey: string };

export interface CredentialCard {
  id: string;
  kind: CredentialKind;
  /** Grouping id, e.g. 'claude', 'opencodeFree', or 'oauth:antigravity'. */
  groupKey: string;
  brand: ProviderBrand;
  title: string;
  subtitle: string | null;
  /** The credential itself is disabled — all rows are locked. */
  lockDisabled: boolean;
  /** Config key entry carries the wildcard disable-all rule (`*`). */
  disableAll: boolean;
  /** Model names currently configured (key allowlist or OpenAI-compatible model list). */
  configuredIds: string[];
  /** Base exclusion rules from the credential config (non-OAuth cards). */
  excludedRules: string[];
  /** Alias overrides keyed by model name. */
  aliases: Record<string, string>;
  /** Null when the credential cannot run upstream discovery (e.g. vertex). */
  discovery: UpstreamDiscoveryInput | null;
  mutation: CredentialMutation;
}

export interface ProviderCardGroup {
  id: string;
  brand: ProviderBrand;
  cards: CredentialCard[];
}

/** Where the model name came from. */
export type ModelRowOrigin = 'configured' | 'discovered' | 'catalog' | 'rule';

export interface ModelRow {
  id: string;
  alias?: string;
  origin: ModelRowOrigin;
  /** The model is currently unavailable for this credential. */
  excluded: boolean;
  /** The toggle is not permitted for this row. */
  locked: boolean;
  wildcardRule?: string;
  globalRule?: string;
  /** Merged OAuth group only: rule from individual auth files, not editable there. */
  fileOnlyRule?: string;
}

export type RowLockReason = 'wildcard' | 'global' | 'unknown-catalog' | undefined;

const trimTrailingSlash = (value?: string | null): string =>
  (value ?? '').trim().replace(/\/+$/, '');

const aliasMap = (models?: ModelAlias[]): Record<string, string> => {
  const map: Record<string, string> = {};
  for (const model of models ?? []) {
    if (model?.name && model.alias && model.alias !== model.name) {
      map[model.name] = model.alias;
    }
  }
  return map;
};

const buildConfigKeyCard = (
  brand: ConfigKeyBrand,
  groupKey: string,
  raw: GeminiKeyConfig | ProviderKeyConfig,
  index: number
): CredentialCard => {
  // Multi-key credentials (Claude) share one model configuration; the plain
  // api-key is empty and discovery uses the first key entry instead.
  const keyEntries = (raw as ProviderKeyConfig).apiKeyEntries ?? [];
  const firstEntryKey = keyEntries.find((entry) => entry.apiKey.trim())?.apiKey;
  const title =
    maskApiKey(raw.apiKey) ||
    (firstEntryKey ? `${maskApiKey(firstEntryKey)} +${keyEntries.length - 1}` : '(keyless)');
  return {
    id: `key:${brand}:${index}`,
    kind: 'config-key',
    groupKey,
    brand,
    title,
    subtitle: trimTrailingSlash(raw.baseUrl) || null,
    lockDisabled: false,
    disableAll: hasExcludedRule(normalizeExcludedRules(raw.excludedModels ?? []), DISABLE_ALL_RULE),
    configuredIds: (raw.models ?? []).map((model) => model.name).filter(Boolean),
    excludedRules: normalizeExcludedRules(raw.excludedModels ?? []),
    aliases: aliasMap(raw.models),
    discovery: isUpstreamDiscoveryBrand(brand)
      ? {
          brand,
          baseUrl: raw.baseUrl,
          apiKey: raw.apiKey || firstEntryKey,
          authIndex: raw.authIndex,
          headers: raw.headers,
        }
      : null,
    mutation: {
      type: 'config-key',
      keyBrand: brand,
      index,
      apiKey: (raw.apiKey ?? '').trim() || null,
    },
  };
};

const buildOpenAICompatCard = (
  groupKey: string,
  raw: OpenAIProviderConfig,
  index: number
): CredentialCard => ({
  id: `openai:${index}`,
  kind: 'openai-compat',
  groupKey,
  brand: 'openaiCompatibility',
  title: (raw.name ?? '').trim() || `Provider ${index + 1}`,
  subtitle: trimTrailingSlash(raw.baseUrl) || null,
  lockDisabled: false,
  disableAll: false,
  configuredIds: (raw.models ?? []).map((model) => model.name).filter(Boolean),
  excludedRules: [],
  aliases: aliasMap(raw.models),
  discovery: {
    brand: 'openaiCompatibility',
    baseUrl: raw.baseUrl,
    apiKeyEntries: raw.apiKeyEntries,
  },
  mutation: {
    type: 'openai-compat',
    name: raw.name ?? '',
    index,
    models: raw.models ?? [],
  },
});

const groupForOpenAIEntry = (config: OpenAIProviderConfig): string => {
  if (isOpencodeFreeOpenAIProvider(config)) return 'opencodeFree';
  if (isQiniuCloudOpenAIProvider(config)) return 'qiniuCloud';
  if (isKimiOpenAIProvider(config)) return 'kimi';
  return 'openaiCompatibility';
};

/**
 * Builds provider credential cards from the normalized config state.
 * Sponsor providers stay grouped so their protocol entries share one card.
 */
export const groupConfigCredentials = (config: Config): ProviderCardGroup[] => {
  const groups = new Map<string, { brand: ProviderBrand; cards: CredentialCard[] }>();
  const push = (card: CredentialCard, fallbackBrand: ProviderBrand) => {
    const group = groups.get(card.groupKey) ?? { brand: fallbackBrand, cards: [] };
    groups.set(card.groupKey, group);
    group.cards.push(card);
  };

  (config.interactionsApiKeys ?? []).forEach((raw, index) => {
    push(buildConfigKeyCard('interactions', 'interactions', raw, index), 'interactions');
  });
  (config.codexApiKeys ?? []).forEach((raw, index) => {
    let groupKey = 'codex';
    if (isFennoAICodexProvider(raw)) groupKey = 'fennoAI';
    else if (isQiniuCloudCodexProvider(raw)) groupKey = 'qiniuCloud';
    else if (isKimiCodexProvider(raw)) groupKey = 'kimi';
    push(buildConfigKeyCard('codex', groupKey, raw, index), 'codex');
  });
  (config.claudeApiKeys ?? []).forEach((raw, index) => {
    let groupKey = 'claude';
    if (isFennoAIClaudeProvider(raw)) groupKey = 'fennoAI';
    else if (isQiniuCloudClaudeProvider(raw)) groupKey = 'qiniuCloud';
    else if (isKimiClaudeProvider(raw)) groupKey = 'kimi';
    push(buildConfigKeyCard('claude', groupKey, raw, index), 'claude');
  });
  (config.geminiApiKeys ?? []).forEach((raw, index) => {
    const groupKey = isQiniuCloudGeminiProvider(raw) ? 'qiniuCloud' : 'gemini';
    push(buildConfigKeyCard('gemini', groupKey, raw, index), 'gemini');
  });
  (config.metaApiKeys ?? []).forEach((raw, index) => {
    push(buildConfigKeyCard('meta', 'meta', raw, index), 'meta');
  });
  (config.xaiApiKeys ?? []).forEach((raw, index) => {
    push(buildConfigKeyCard('xai', 'xai', raw, index), 'xai');
  });
  (config.vertexApiKeys ?? []).forEach((raw, index) => {
    push(buildConfigKeyCard('vertex', 'vertex', raw, index), 'vertex');
  });
  (config.openaiCompatibility ?? []).forEach((raw, index) => {
    push(buildOpenAICompatCard(groupForOpenAIEntry(raw), raw, index), 'openaiCompatibility');
  });

  return Array.from(groups.entries()).map(([id, group]) => ({
    id,
    brand: group.brand,
    cards: group.cards,
  }));
};

export interface BuildModelRowsArgs {
  card: CredentialCard;
  /** Upstream catalog fetched via discovery (non-OAuth cards). */
  discoveredNames?: string[] | null;
  /** Static registry catalog (OAuth cards). */
  catalogNames?: string[] | null;
  /** Per-credential exclusion rules (OAuth: fetched from the auth file). */
  fileRules?: string[];
  /** Global per-provider OAuth exclusion rules, keyed by provider. */
  globalRulesByProvider?: Record<string, string[]>;
  oauthProviderKey?: string;
  /** Config key entry prefixes its models — allowlist-aware hint row state. */
  catalogPending?: boolean;
}

/**
 * Merges every configured, discovered, static-catalog, and rule-only model
 * name into a deduplicated, ordered row list with availability state.
 *
 * Availability semantics differ by credential kind:
 * - `openai-compat`: the `models` list is the only control, so a model is on
 *   exactly when it has a configured entry.
 * - `config-key` / `oauth-file`: models are on unless excluded by an exact
 *   rule, a wildcard rule, or (OAuth) a matching global provider rule.
 */
export const buildModelRows = (args: BuildModelRowsArgs): ModelRow[] => {
  const { card } = args;

  if (card.kind === 'openai-compat') {
    const configured = new Set(card.configuredIds.map((id) => id.toLowerCase()));
    const rows = new Map<string, ModelRow>();
    const addRow = (name: string, origin: ModelRowOrigin) => {
      const key = name.toLowerCase();
      if (!key || rows.has(key)) return;
      rows.set(key, {
        id: name,
        alias: card.aliases[name],
        origin,
        excluded: !configured.has(key),
        locked: false,
      });
    };
    card.configuredIds.forEach((name) => addRow(name, 'configured'));
    (args.discoveredNames ?? []).forEach((name) => addRow(name, 'discovered'));
    return Array.from(rows.values());
  }

  const rules = normalizeExcludedRules(args.fileRules ?? card.excludedRules);
  const globalRules =
    card.kind === 'oauth-file' || card.kind === 'oauth-group'
      ? normalizeExcludedRules(
          args.globalRulesByProvider?.[(args.oauthProviderKey ?? '').trim()] ?? []
        )
      : [];

  let order = 0;
  const rows = new Map<string, { row: ModelRow; order: number }>();
  const addRow = (name: string, origin: ModelRowOrigin) => {
    const key = name.toLowerCase();
    if (!key || rows.has(key)) return;
    const wildcardRule = isMatchedByWildcardRule(rules, name)
      ? rules.find((rule) => isWildcardRule(rule) && matchesExcludedRule(rule, name))
      : undefined;
    const globalRule = globalRules.find((rule) => matchesExcludedRule(rule, name));
    const excluded = hasExcludedRule(rules, name) || Boolean(wildcardRule) || Boolean(globalRule);
    const locked =
      card.lockDisabled || card.disableAll || Boolean(wildcardRule) || Boolean(globalRule);
    // Merged OAuth group cards toggle the global rules layer. A model excluded
    // only by individual author files stays off no matter what the group toggle
    // writes, so it is locked with a dedicated hint instead of silently
    // ignoring the request.
    const fileOnlyRule =
      card.kind === 'oauth-group' && excluded && !wildcardRule && !globalRule
        ? hasExcludedRule(rules, name)
          ? name
          : undefined
        : undefined;
    rows.set(key, {
      row: {
        id: name,
        alias: card.aliases[name],
        origin,
        excluded,
        locked: locked || Boolean(fileOnlyRule),
        wildcardRule: wildcardRule || undefined,
        globalRule: globalRule || undefined,
        fileOnlyRule: fileOnlyRule || undefined,
      },
      order: order++,
    });
  };

  card.configuredIds.forEach((name) => addRow(name, 'configured'));
  (args.discoveredNames ?? []).forEach((name) => addRow(name, 'discovered'));
  (args.catalogNames ?? []).forEach((name) => addRow(name, 'catalog'));
  // Rules without a matching candidate still surface so nothing is hidden.
  rules.forEach((rule) => {
    if (isWildcardRule(rule) || rule === DISABLE_ALL_RULE) return;
    addRow(rule, 'rule');
  });
  globalRules.forEach((rule) => {
    if (isWildcardRule(rule)) return;
    addRow(rule, 'rule');
  });

  return Array.from(rows.values())
    .sort((left, right) => left.order - right.order)
    .map((entry) => entry.row);
};

/** Toggles one exact model rule, keeping wildcard rules untouched. */
export const toggleExactExclusion = (
  rules: readonly string[],
  modelId: string,
  exclude: boolean
): string[] => {
  if (exclude && isWildcardRule(modelId)) return normalizeExcludedRules(rules);
  return toggleExcludedRule(normalizeExcludedRules(rules), modelId, exclude);
};
