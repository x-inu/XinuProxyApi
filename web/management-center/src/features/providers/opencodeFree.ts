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
 * Free models on the OpenCode catalog carry a `-free` id suffix; the
 * endpoint does not report pricing, so the suffix is the only marker.
 */
export const isOpenCodeFreeModelId = (name: string | undefined | null): boolean =>
  /-free$/i.test(String(name ?? '').trim());

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
