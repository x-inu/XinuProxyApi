import type { Config, OpenAIProviderConfig } from '@/types';
import type { SponsorProviderRaw } from './types';

export const OPENCODE_FREE_PROVIDER_NAME = 'opencodeFree';
export const OPENCODE_FREE_DISPLAY_NAME = 'OpenCode Free';
export const OPENCODE_FREE_BASE_URL = 'https://opencode.ai/zen';
export const OPENCODE_FREE_OPENAI_BASE_URL = `${OPENCODE_FREE_BASE_URL}/v1`;

export const OPENCODE_FREE_BASE_URL_OPTIONS = [
  {
    id: 'zen',
    descriptionKey: 'zen',
    baseUrl: OPENCODE_FREE_BASE_URL,
    openaiBaseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
    codexBaseUrl: '',
    anthropicBaseUrl: '',
    geminiBaseUrl: '',
  },
] as const;

export const OPENCODE_FREE_PROTOCOLS = ['openai'] as const;

const normalizeText = (value: string | undefined | null): string =>
  String(value ?? '')
    .trim()
    .toLowerCase();

const normalizeBaseUrl = (value: string | undefined | null): string =>
  normalizeText(value).replace(/\/+$/, '');

export const resolveOpencodeFreeBaseUrl = (value: string | undefined | null): string => {
  const normalized = normalizeBaseUrl(value);
  const matched = OPENCODE_FREE_BASE_URL_OPTIONS.find(
    (option) =>
      normalized === normalizeBaseUrl(option.baseUrl) ||
      normalized === normalizeBaseUrl(option.openaiBaseUrl)
  );
  return matched?.baseUrl ?? OPENCODE_FREE_BASE_URL;
};

export const getOpencodeFreeProtocolUrls = (value: string | undefined | null) => {
  const baseUrl = resolveOpencodeFreeBaseUrl(value);
  const matched =
    OPENCODE_FREE_BASE_URL_OPTIONS.find(
      (option) => normalizeBaseUrl(option.baseUrl) === normalizeBaseUrl(baseUrl)
    ) ?? OPENCODE_FREE_BASE_URL_OPTIONS[0];
  return {
    anthropic: matched.anthropicBaseUrl,
    openai: matched.openaiBaseUrl,
    codex: matched.codexBaseUrl,
    gemini: matched.geminiBaseUrl,
  };
};

export const isOpencodeFreeOpenAIProvider = (
  config: OpenAIProviderConfig | undefined | null
): boolean => {
  if (!config) return false;
  const normalized = normalizeBaseUrl(config.baseUrl);
  return OPENCODE_FREE_BASE_URL_OPTIONS.some(
    (option) =>
      normalized === normalizeBaseUrl(option.openaiBaseUrl) ||
      normalized === normalizeBaseUrl(option.baseUrl)
  );
};

/**
 * Fallback marker for OpenCode free ids. The catalog now reports pricing
 * (the v2 `cost` block), so the suffix only matters for payloads without a
 * usable price block — never on top of a confirmed free/paid flag.
 */
export const isOpenCodeFreeModelId = (name: string | undefined | null): boolean =>
  /-free$/i.test(String(name ?? '').trim());

/**
 * Free models curated by OpenCode in the Console docs — the pinned minimum
 * for the OpenCode (free) card, independent of what the live catalog serves
 * right now:
 * https://opencode.ai/v2/docs/console/models/#free-models
 *
 * Some ids may be missing from `https://opencode.ai/zen/v1/models` (not
 * served upstream yet); those surface on the Models page as pending rows.
 */
export const OPENCODE_CONSOLE_FREE_MODEL_IDS: readonly string[] = [
  'beam-free',
  'big-pickle',
  'deepseek-v4.1-flash-free',
  'flint-40b-free',
  'glm-5.3-flash-free',
  'jev-1.13-free',
  'mimo-v2.6-flash-free',
  'nemotron-3-ultra-free',
  'nemotron-3.5-lightning-free',
  'nemotron-3.5-super-vl-free',
];

export const buildOpencodeFreeRaw = (config: Config | null | undefined): SponsorProviderRaw => ({
  openai: (config?.openaiCompatibility ?? [])
    .map((item, index) => ({ config: item, index: item.sourceIndex ?? index }))
    .filter((item) => isOpencodeFreeOpenAIProvider(item.config)),
  claude: [],
  codex: [],
  gemini: [],
});

export const hasOpencodeFreeConfig = (config: Config | null | undefined): boolean => {
  const raw = buildOpencodeFreeRaw(config);
  return raw.openai.length > 0;
};
