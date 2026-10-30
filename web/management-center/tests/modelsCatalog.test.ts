import { describe, expect, test } from 'bun:test';
import type { Config, OpenAIProviderConfig } from '../src/types';
import {
  applyFreeOnlyFilter,
  buildModelRows,
  groupConfigCredentials,
  toggleExactExclusion,
  type CredentialCard,
  type ModelRow,
} from '../src/features/models/modelsCatalog';
import {
  OPENCODE_FREE_PROVIDER_NAME,
  OPENCODE_FREE_OPENAI_BASE_URL,
} from '../src/features/providers/opencodeFree';
import { FENNO_AI_ANTHROPIC_BASE_URL } from '../src/features/providers/fennoAI';

describe('groupConfigCredentials', () => {
  test('groups generic sections by brand and detects sponsor entries', () => {
    const config: Config = {
      claudeApiKeys: [
        { apiKey: 'claude-key-1', baseUrl: 'https://api.anthropic.com' },
        { apiKey: 'fenno-key', baseUrl: FENNO_AI_ANTHROPIC_BASE_URL },
      ],
      openaiCompatibility: [
        {
          name: OPENCODE_FREE_PROVIDER_NAME,
          baseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
        } as OpenAIProviderConfig,
        {
          name: 'My Relay',
          baseUrl: 'https://relay.example.com/v1',
        },
      ],
    };

    const groups = groupConfigCredentials(config);
    const byId = new Map(groups.map((group) => [group.id, group]));

    expect(byId.get('claude')?.cards).toHaveLength(1);
    expect(byId.get('fennoAI')?.cards).toHaveLength(1);
    expect(byId.get('opencodeFree')?.cards).toHaveLength(1);
    expect(byId.get('openaiCompatibility')?.cards).toHaveLength(1);
    // Sponsor groups surface their own brand so the Models page shows the
    // provider name and logo instead of the generic entry-type label.
    expect(byId.get('opencodeFree')?.brand).toBe('opencodeFree');
    expect(byId.get('fennoAI')?.brand).toBe('fennoAI');
    expect(byId.get('openaiCompatibility')?.brand).toBe('openaiCompatibility');
  });

  test('key card titles mask api keys and keep the disable-all flag', () => {
    const config: Config = {
      geminiApiKeys: [
        {
          apiKey: 'AIzaSuperSecretKey123456',
          excludedModels: ['*'],
        },
      ],
    };

    const [group] = groupConfigCredentials(config);
    const card = group.cards[0];
    expect(card.title.includes('AIzaSuperSecretKey123456')).toBe(false);
    expect(card.disableAll).toBe(true);
    expect(card.id).toBe('key:gemini:0');
    expect(card.excludedRules).toEqual(['*']);
  });
});

const configCard = (overrides: Partial<CredentialCard> = {}): CredentialCard => ({
  id: 'key:claude:0',
  kind: 'config-key',
  groupKey: 'claude',
  brand: 'claude',
  title: 'sk-…abcd',
  subtitle: null,
  lockDisabled: false,
  disableAll: false,
  configuredIds: ['claude-sonnet-4-5'],
  excludedRules: [],
  aliases: {},
  discovery: null,
  mutation: {
    type: 'config-key',
    keyBrand: 'claude',
    index: 0,
    apiKey: 'sk-key',
  },
  ...overrides,
});

describe('buildModelRows', () => {
  test('config-key cards merge configured, discovered, and rule-only models', () => {
    const card = configCard({
      excludedRules: ['legacy-model'],
    });
    const rows = buildModelRows({
      card,
      discoveredNames: ['claude-opus-4', 'claude-sonnet-4-5'],
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('claude-sonnet-4-5')?.excluded).toBe(false);
    expect(byId.get('claude-sonnet-4-5')?.origin).toBe('configured');
    expect(byId.get('claude-opus-4')?.origin).toBe('discovered');
    expect(byId.get('legacy-model')?.origin).toBe('rule');
    expect(byId.get('legacy-model')?.excluded).toBe(true);
    expect(rows).toHaveLength(3);
  });

  test('wildcard rules exclude and lock matching rows', () => {
    const card = configCard({
      configuredIds: ['gemini-2.5-pro', 'gemini-2.5-flash'],
      excludedRules: ['gemini-2.5-flash*'],
    });
    const rows = buildModelRows({ card });
    const flash = rows.find((row) => row.id === 'gemini-2.5-flash');
    expect(flash?.excluded).toBe(true);
    expect(flash?.locked).toBe(true);
    expect(flash?.wildcardRule).toBe('gemini-2.5-flash*');
  });

  test('disable-all rule locks every row', () => {
    const card = configCard({
      configuredIds: ['model-a'],
      excludedRules: ['*'],
      disableAll: true,
    });
    const rows = buildModelRows({ card });
    expect(rows.every((row) => row.locked)).toBe(true);
    expect(rows.every((row) => row.excluded)).toBe(true);
  });

  test('openai-compat rows are on exactly for configured names', () => {
    const card = configCard({
      kind: 'openai-compat',
      id: 'openai:0',
      groupKey: 'openaiCompatibility',
      brand: 'openaiCompatibility',
      configuredIds: ['model-a', 'model-b'],
      aliases: { 'model-b': 'b-alias' },
    });
    const rows = buildModelRows({
      card,
      discoveredNames: ['model-c'],
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('model-a')?.excluded).toBe(false);
    expect(byId.get('model-b')?.excluded).toBe(false);
    expect(byId.get('model-b')?.alias).toBe('b-alias');
    expect(byId.get('model-c')?.excluded).toBe(true);
    expect(byId.get('model-c')?.origin).toBe('discovered');
  });

  test('oauth rows honor per-file and global rules and lock global-rule rows', () => {
    const card: CredentialCard = {
      id: 'auth:account.json',
      kind: 'oauth-file',
      groupKey: 'oauth:antigravity',
      brand: 'openaiCompatibility',
      title: 'account.json',
      subtitle: null,
      lockDisabled: false,
      disableAll: false,
      configuredIds: [],
      excludedRules: [],
      aliases: {},
      discovery: null,
      mutation: { type: 'oauth-file', fileName: 'account.json' },
    };
    const rows = buildModelRows({
      card,
      catalogNames: ['model-a', 'model-b'],
      fileRules: ['old-invalid-model'],
      globalRulesByProvider: { antigravity: ['model-b'] },
      oauthProviderKey: 'antigravity',
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('model-a')?.excluded).toBe(false);
    expect(byId.get('model-a')?.origin).toBe('catalog');
    expect(byId.get('model-b')?.excluded).toBe(true);
    expect(byId.get('model-b')?.locked).toBe(true);
    expect(byId.get('old-invalid-model')?.origin).toBe('rule');
  });
});

describe('merged oauth group cards', () => {
  const groupCard: CredentialCard = {
    id: 'auth-group:antigravity',
    kind: 'oauth-group',
    groupKey: 'oauth:antigravity',
    brand: 'openaiCompatibility',
    title: 'antigravity',
    subtitle: '20 accounts · one rules set',
    lockDisabled: false,
    disableAll: false,
    configuredIds: [],
    excludedRules: [],
    aliases: {},
    discovery: null,
    mutation: { type: 'oauth-group', providerKey: 'antigravity' },
  };

  test('global rules drive exclusion and stay toggleable via the group card', () => {
    const rows = buildModelRows({
      card: groupCard,
      catalogNames: ['model-a', 'model-b'],
      fileRules: [],
      globalRulesByProvider: { antigravity: ['model-b'] },
      oauthProviderKey: 'antigravity',
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('model-a')?.excluded).toBe(false);
    expect(byId.get('model-a')?.locked).toBe(false);
    expect(byId.get('model-b')?.excluded).toBe(true);
    // The group card owns the global layer, so the row must stay editable:
    // locking it would make a model disabled here impossible to re-enable.
    expect(byId.get('model-b')?.locked).toBe(false);
    expect(byId.get('model-b')?.globalRule).toBe('model-b');
    expect(byId.get('model-b')?.fileOnlyRule).toBeUndefined();
  });

  test('a global rule combined with a per-account rule stays locked', () => {
    const rows = buildModelRows({
      card: groupCard,
      catalogNames: ['model-a', 'model-b'],
      fileRules: ['model-b'],
      globalRulesByProvider: { antigravity: ['model-b'] },
      oauthProviderKey: 'antigravity',
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('model-b')?.excluded).toBe(true);
    // Clearing the global rule would not enable it: the per-account rule
    // still applies, so the toggle stays locked with the per-account hint.
    expect(byId.get('model-b')?.locked).toBe(true);
    expect(byId.get('model-b')?.fileOnlyRule).toBe('model-b');
    expect(byId.get('model-b')?.globalRule).toBe('model-b');
  });

  test('per-file-only rules lock the row with a dedicated hint', () => {
    const rows = buildModelRows({
      card: groupCard,
      catalogNames: ['model-a', 'model-b', 'model-c'],
      fileRules: ['old-invalid-model'],
      globalRulesByProvider: { antigravity: ['model-b'] },
      oauthProviderKey: 'antigravity',
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('old-invalid-model')?.excluded).toBe(true);
    expect(byId.get('old-invalid-model')?.locked).toBe(true);
    expect(byId.get('old-invalid-model')?.fileOnlyRule).toBe('old-invalid-model');
    expect(byId.get('old-invalid-model')?.origin).toBe('rule');
    expect(byId.get('model-b')?.fileOnlyRule).toBeUndefined();
    expect(byId.get('model-c')?.excluded).toBe(false);
  });

  test('wildcard file rules stay locked and flagged separately', () => {
    const rows = buildModelRows({
      card: groupCard,
      catalogNames: ['claude-x'],
      fileRules: ['claude-*'],
      globalRulesByProvider: {},
      oauthProviderKey: 'antigravity',
    });

    const byId = new Map<string, ModelRow>(rows.map((row) => [row.id, row]));
    expect(byId.get('claude-x')?.excluded).toBe(true);
    expect(byId.get('claude-x')?.locked).toBe(true);
    expect(byId.get('claude-x')?.wildcardRule).toBe('claude-*');
    expect(byId.get('claude-x')?.fileOnlyRule).toBeUndefined();
  });
});

describe('applyFreeOnlyFilter', () => {
  const catalog = [
    { name: 'free-model-a', isFree: true },
    { name: 'paid-model-b', isFree: false },
    { name: 'unknown-pricing-c' },
    { name: 'free-model-d', isFree: true },
  ];

  test('keeps explicit free models and unknown pricing, drops confirmed paid', () => {
    expect(applyFreeOnlyFilter(catalog, true).map((model) => model.name)).toEqual([
      'free-model-a',
      'unknown-pricing-c',
      'free-model-d',
    ]);
  });

  test('off keeps every entry', () => {
    expect(applyFreeOnlyFilter(catalog, false)).toHaveLength(4);
  });

  test('suffix fallback narrows unpriced ids for OpenCode', () => {
    const opencode = [
      { name: 'glm-5.3-flash-free' },
      { name: 'claude-opus-5-5' },
      { name: 'beam-free' },
      { name: 'test' },
      { name: 'priced-free', isFree: true },
      { name: 'priced-paid', isFree: false },
    ];
    expect(
      applyFreeOnlyFilter(opencode, true, { inferSuffixFree: true }).map((model) => model.name)
    ).toEqual(['glm-5.3-flash-free', 'beam-free', 'priced-free']);
  });
});

describe('toggleExactExclusion', () => {  test('adds and removes exact rules while keeping wildcards', () => {
    const base = ['gpt-*', 'claude-opus'];
    const disabled = toggleExactExclusion(base, 'claude-sonnet', true);
    expect(disabled).toEqual(['gpt-*', 'claude-opus', 'claude-sonnet']);

    const enabledAgain = toggleExactExclusion(disabled, 'claude-sonnet', false);
    expect(enabledAgain).toEqual(base);

    const removedExact = toggleExactExclusion(base, 'claude-opus', false);
    expect(removedExact).toEqual(['gpt-*']);
  });

  test('never adds a wildcard model id as an exclusion rule', () => {
    const next = toggleExactExclusion(['gpt-*'], 'kimi-*', true);
    expect(next).toEqual(['gpt-*']);
  });
});
