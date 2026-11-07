import type { ComponentType } from 'react';
import {
  IconCode,
  IconKey,
  IconNetwork,
  IconSatellite,
  IconScrollText,
  IconShield,
  IconSlidersHorizontal,
  IconTimer,
  type IconProps,
} from '@/components/ui/icons';
import type { VisualConfigFieldPath } from '@/types/visualConfig';
import type { VisualSectionId } from './searchIndex';


export type ConfigEditorMode = 'visual' | 'source';


export type ConfigTabId = 'common' | VisualSectionId;

export const CONFIG_SECTION_IDS = [
  'connectivity',
  'network',
  'logging',
  'quota',
  'streaming',
  'advanced',
  'payload',
] as const satisfies readonly VisualSectionId[];

export const CONFIG_TAB_IDS: readonly ConfigTabId[] = ['common', ...CONFIG_SECTION_IDS];


export const SECTION_INDEX_LABELS: Record<VisualSectionId, string> = {
  connectivity: '01',
  network: '02',
  logging: '03',
  quota: '04',
  streaming: '05',
  advanced: '06',
  payload: '07',
};

export const CONFIG_TAB_ICONS: Record<ConfigTabId, ComponentType<IconProps>> = {
  common: IconSlidersHorizontal,
  connectivity: IconKey,
  network: IconNetwork,
  logging: IconScrollText,
  quota: IconTimer,
  streaming: IconSatellite,
  advanced: IconShield,
  payload: IconCode,
};


export const COMMON_FIELD_IDS = [
  'host',
  'port',
  'apiKeys',
  'proxyUrl',
  'debug',
  'loggingToFile',
  'quotaSwitchProject',
  'quotaSwitchPreviewModel',
] as const;


export const SECTION_VALIDATION_FIELDS: Record<VisualSectionId, readonly VisualConfigFieldPath[]> =
  {
    connectivity: ['port'],
    network: ['requestRetry', 'maxRetryCredentials', 'maxRetryInterval', 'authAutoRefreshWorkers'],
    logging: ['errorLogsMaxFiles', 'logsMaxTotalSizeMb', 'redisUsageQueueRetentionSeconds'],
    quota: [],
    streaming: [
      'streaming.keepaliveSeconds',
      'streaming.bootstrapRetries',
      'streaming.nonstreamKeepaliveInterval',
    ],
    advanced: [],
    payload: [],
  };


export const FIELD_VALUE_KEYS: Record<string, readonly string[]> = {
  // ── connectivity ──────────────────────────────────────────────────────────
  host: ['host'],
  port: ['port'],
  authDir: ['authDir'],
  apiKeys: ['apiKeysText'],
  publicModelsEndpoint: ['publicModelsEndpoint'],
  tlsEnable: ['tlsEnable'],
  tlsCert: ['tlsCert'],
  tlsKey: ['tlsKey'],
  rmAllowRemote: ['rmAllowRemote'],
  rmDisableControlPanel: ['rmDisableControlPanel'],
  rmDisableAutoUpdatePanel: ['rmDisableAutoUpdatePanel'],
  rmSecretKey: ['rmSecretKey'],
  rmPanelRepo: ['rmPanelRepo'],
  // ── network ───────────────────────────────────────────────────────────────
  proxyUrl: ['proxyUrl'],
  requestRetry: ['requestRetry'],
  maxRetryCredentials: ['maxRetryCredentials'],
  maxRetryInterval: ['maxRetryInterval'],
  authAutoRefreshWorkers: ['authAutoRefreshWorkers'],
  routingStrategy: ['routingStrategy'],
  disableImageGeneration: ['disableImageGeneration'],
  gptImage2BaseModel: ['gptImage2BaseModel'],
  routingSessionAffinityTTL: ['routingSessionAffinityTTL'],
  forceModelPrefix: ['forceModelPrefix'],
  passthroughHeaders: ['passthroughHeaders'],
  disableCooling: ['disableCooling'],
  routingSessionAffinity: ['routingSessionAffinity'],
  wsAuth: ['wsAuth'],
  // ── logging ───────────────────────────────────────────────────────────────
  debug: ['debug'],
  commercialMode: ['commercialMode'],
  loggingToFile: ['loggingToFile'],
  logsMaxTotalSizeMb: ['logsMaxTotalSizeMb'],
  errorLogsMaxFiles: ['errorLogsMaxFiles'],
  redisUsageQueueRetentionSeconds: ['redisUsageQueueRetentionSeconds'],
  usageStatisticsEnabled: ['usageStatisticsEnabled'],
  // ── quota ─────────────────────────────────────────────────────────────────
  quotaSwitchProject: ['quotaSwitchProject'],
  quotaSwitchPreviewModel: ['quotaSwitchPreviewModel'],
  quotaAntigravityCredits: ['quotaAntigravityCredits'],
  // ── streaming ─────────────────────────────────────────────────────────────
  streamingKeepaliveSeconds: ['streaming.keepaliveSeconds'],
  streamingBootstrapRetries: ['streaming.bootstrapRetries'],
  streamingNonstreamKeepalive: ['streaming.nonstreamKeepaliveInterval'],
  // ── advanced ──────────────────────────────────────────────────────────────
  pluginsEnabled: ['pluginsEnabled'],
  pluginStoreSources: ['pluginStoreSources'],
  pluginStoreAuth: ['pluginStoreAuth'],
  antigravitySensitiveWords: ['antigravitySensitiveWords'],
  devinSensitiveWords: ['devinSensitiveWords'],
  antigravitySignatureCacheEnabled: ['antigravitySignatureCacheEnabled'],
  antigravitySignatureBypassStrict: ['antigravitySignatureBypassStrict'],
  claudeHeaderUserAgent: ['claudeHeaderUserAgent'],
  claudeHeaderPackageVersion: ['claudeHeaderPackageVersion'],
  claudeHeaderRuntimeVersion: ['claudeHeaderRuntimeVersion'],
  claudeHeaderOs: ['claudeHeaderOs'],
  claudeHeaderArch: ['claudeHeaderArch'],
  claudeHeaderTimeout: ['claudeHeaderTimeout'],
  claudeHeaderStabilizeDeviceProfile: ['claudeHeaderStabilizeDeviceProfile'],
  codexHeaderUserAgent: ['codexHeaderUserAgent'],
  codexHeaderBetaFeatures: ['codexHeaderBetaFeatures'],
  // ── payload ───────────────────────────────────────────────────────────────
  payloadDefaultRules: ['payloadDefaultRules'],
  payloadDefaultRawRules: ['payloadDefaultRawRules'],
  payloadOverrideRules: ['payloadOverrideRules'],
  payloadOverrideRawRules: ['payloadOverrideRawRules'],
  payloadFilterRules: ['payloadFilterRules'],
};


export const configTabDomId = (id: ConfigTabId) => `config-tab-${id}`;
export const configPanelDomId = (id: ConfigTabId) => `config-panel-${id}`;


export const CONFIG_MODE_STORAGE_KEY = 'config-management:tab';
export const CONFIG_SECTION_STORAGE_KEY = 'config-management:section';

export const LEGACY_EDITOR_MODE_STORAGE_KEY = 'config-management:editor-mode';
