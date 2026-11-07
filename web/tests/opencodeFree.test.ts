import { afterEach, describe, expect, test } from 'bun:test';
import { apiClient } from '../src/services/api/client';
import { providersApi } from '../src/services/api/providers';
import { normalizeOpenAIProvider } from '../src/services/api/transformers';
import {
  appendOpencodeFreeSlot,
  opencodeFreeToResource,
  opencodeFreeToResources,
} from '../src/features/providers/adapters';
import {
  buildOpencodeFreeRaw,
  OPENCODE_CONSOLE_FREE_MODEL_IDS,
  OPENCODE_FREE_OPENAI_BASE_URL,
  OPENCODE_FREE_PROVIDER_NAME,
} from '../src/features/providers/opencodeFree';
import { getSponsorAggregationConflict } from '../src/features/providers/sponsorDefinitions';
import { PROVIDER_LOGOS } from '../src/features/providers/brandLogos';

/** Backend (kebab-case) shape as served by the management API. */
const backendEntry = (proxyUrl?: string, weight?: number) => ({
  name: OPENCODE_FREE_PROVIDER_NAME,
  'base-url': OPENCODE_FREE_OPENAI_BASE_URL,
  prefix: 'oc',
  'synthesize-opencode-identity': true,
  'api-key-entries': [
    proxyUrl || weight !== undefined
      ? { 'api-key': '', 'proxy-url': proxyUrl, weight }
      : { 'api-key': '' },
    { 'api-key': '' },
  ],
  models: [{ name: 'space-bunny-free', alias: 'space-bunny-free' }],
});

/** Normalized store shape (camelCase) as consumed by the workbench. */
const normalizedEntry = (proxyUrl?: string, weight?: number) => ({
  name: OPENCODE_FREE_PROVIDER_NAME,
  baseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
  prefix: 'oc',
  synthesizeOpencodeIdentity: true,
  apiKeyEntries: [
    { apiKey: '', proxyUrl, weight },
    { apiKey: '' },
  ],
  models: [{ name: 'space-bunny-free', alias: 'space-bunny-free' }],
  sourceIndex: 2,
});

const originalGet = apiClient.get;
const originalPut = apiClient.put;

afterEach(() => {
  apiClient.get = originalGet;
  apiClient.put = originalPut;
});

describe('opencodeFree phase 2', () => {
  test('matches keyless entries by base URL', () => {
    const raw = buildOpencodeFreeRaw({
      openaiCompatibility: [normalizedEntry('http://127.0.0.1:7890', 2)],
    });
    expect(raw.openai).toHaveLength(1);
    expect(raw.claude).toEqual([]);
    expect(raw.codex).toEqual([]);
  });

  test('builds a resource without an API key preview', () => {
    const raw = buildOpencodeFreeRaw({
      openaiCompatibility: [normalizedEntry()],
    });
    const resource = opencodeFreeToResource(raw);
    expect(resource).not.toBeNull();
    expect(resource?.brand).toBe('opencodeFree');
    expect(resource?.apiKey).toBeNull();
    expect(resource?.apiKeyPreview).toBeNull();
    expect(resource?.apiKeyEntryCount).toBe(2);
  });

  test('keyless multi-slot entries are not an aggregation conflict', () => {
    const raw = buildOpencodeFreeRaw({
      openaiCompatibility: [normalizedEntry()],
    });
    expect(getSponsorAggregationConflict(raw)).toBeNull();
  });

  test('expands one row per rotation slot', () => {
    const entry = normalizedEntry('http://127.0.0.1:7890', 2);
    entry.apiKeyEntries[1] = { apiKey: '', proxyUrl: 'http://127.0.0.1:7891' };
    const rows = opencodeFreeToResources(buildOpencodeFreeRaw({ openaiCompatibility: [entry] }));
    expect(rows).toHaveLength(2);
    expect(rows[0].identifier).toBe('OpenCode Free #1');
    expect(rows[1].identifier).toBe('OpenCode Free #2');
    expect(rows[0].proxyUrl).toBe('http://127.0.0.1:7890');
    expect(rows[1].proxyUrl).toBe('http://127.0.0.1:7891');
    expect(rows[0].id).not.toBe(rows[1].id);
    const sel = rows[0].selector;
    expect(sel.brand).toBe('opencodeFree');
    if (sel.brand === 'opencodeFree') expect(sel.slotIndex).toBe(0);
  });

  test('shows one row for the implicit keyless slot', () => {
    const entry = normalizedEntry();
    delete entry.apiKeyEntries;
    const rows = opencodeFreeToResources(
      buildOpencodeFreeRaw({ openaiCompatibility: [entry] })
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].apiKey).toBeNull();
    expect(rows[0].proxyUrl).toBeNull();
  });

  test('falls back to one row on multiple backend configs', () => {
    const rows = opencodeFreeToResources(
      buildOpencodeFreeRaw({ openaiCompatibility: [normalizedEntry(), normalizedEntry()] })
    );
    expect(rows).toHaveLength(1);
  });

  test('reads the identity synthesis flag', () => {
    expect(
      normalizeOpenAIProvider({
        name: 'x',
        'base-url': 'https://example.com/v1',
        'synthesize-opencode-identity': true,
      })?.synthesizeOpencodeIdentity
    ).toBe(true);
    expect(
      normalizeOpenAIProvider({
        name: 'x',
        'base-url': 'https://example.com/v1',
      })?.synthesizeOpencodeIdentity
    ).toBeUndefined();
  });

  test('keeps keyless slots that carry proxy or weight', () => {
    const entry = backendEntry('http://127.0.0.1:7890', 2);
    // backendEntry already includes one proxy slot and one pure keyless slot.
    entry['api-key-entries'].push({ 'api-key': '', 'proxy-url': 'http://127.0.0.1:7891' });
    const provider = normalizeOpenAIProvider(entry);
    expect(provider?.apiKeyEntries).toHaveLength(3);
    expect(provider?.apiKeyEntries[0]).toMatchObject({
      apiKey: '',
      proxyUrl: 'http://127.0.0.1:7890',
      weight: 2,
    });
    expect(provider?.apiKeyEntries[1]).toMatchObject({ apiKey: '' });
    expect(provider?.apiKeyEntries[2]).toMatchObject({
      apiKey: '',
      proxyUrl: 'http://127.0.0.1:7891',
    });
    expect(provider?.synthesizeOpencodeIdentity).toBe(true);
  });

  test('drops fully empty key entries for standard keyed providers', () => {
    const provider = normalizeOpenAIProvider({
      name: 'x',
      'base-url': 'https://example.com/v1',
      'api-key-entries': [{}, { 'api-key': '  ' }],
    });
    expect(provider?.apiKeyEntries).toEqual([]);
  });

  test('keeps empty key entries for keyless rotation pools', () => {
    const provider = normalizeOpenAIProvider({
      name: OPENCODE_FREE_PROVIDER_NAME,
      'base-url': OPENCODE_FREE_OPENAI_BASE_URL,
      'synthesize-opencode-identity': true,
      'api-key-entries': [{}, { 'api-key': '' }],
    });
    expect(provider?.apiKeyEntries).toHaveLength(2);
    expect(provider?.apiKeyEntries[0]).toMatchObject({ apiKey: '' });
    expect(provider?.apiKeyEntries[1]).toMatchObject({ apiKey: '' });
  });

  test('appends one keyless slot per click', () => {
    const first = appendOpencodeFreeSlot({
      name: OPENCODE_FREE_PROVIDER_NAME,
      baseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
      apiKeyEntries: [],
    });
    // Implicit single slot becomes two explicit slots so the count grows.
    expect(first.apiKeyEntries).toHaveLength(2);
    const once = appendOpencodeFreeSlot({
      name: OPENCODE_FREE_PROVIDER_NAME,
      baseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
      apiKeyEntries: [{ apiKey: '' }],
    });
    expect(once.apiKeyEntries).toHaveLength(2);
    expect(once.apiKeyEntries[1]).toMatchObject({ apiKey: '' });
    expect(once.baseUrl).toBe(OPENCODE_FREE_OPENAI_BASE_URL);
    const twice = appendOpencodeFreeSlot(once);
    expect(twice.apiKeyEntries).toHaveLength(3);
  });

  test('writes the flag and keyless slots with proxy and weight', async () => {
    let written: unknown;
    apiClient.get = (async () => ({
      'openai-compatibility': [backendEntry()],
    })) as typeof apiClient.get;
    apiClient.put = (async (_url: string, data?: unknown) => {
      written = data;
      return undefined;
    }) as typeof apiClient.put;

    await providersApi.updateOpenAIProvider(OPENCODE_FREE_PROVIDER_NAME, 0, {
      name: OPENCODE_FREE_PROVIDER_NAME,
      baseUrl: OPENCODE_FREE_OPENAI_BASE_URL,
      synthesizeOpencodeIdentity: true,
      apiKeyEntries: [
        { apiKey: '', proxyUrl: 'http://127.0.0.1:7890', weight: 2 },
        { apiKey: '' },
      ],
    });

    expect(written).toEqual([
      {
        name: OPENCODE_FREE_PROVIDER_NAME,
        'base-url': OPENCODE_FREE_OPENAI_BASE_URL,
        'synthesize-opencode-identity': true,
        'api-key-entries': [
          { 'api-key': '', 'proxy-url': 'http://127.0.0.1:7890', weight: 2 },
          { 'api-key': '' },
        ],
      },
    ]);
  });

  test('configures official OpenCode brand logo with dark mode variant', () => {
    const logo = PROVIDER_LOGOS.opencodeFree;
    expect(logo).toBeDefined();
    expect(logo.src).toBeTruthy();
    expect(logo.darkSrc).toBeTruthy();
    expect(logo.darkSrc).not.toBe(logo.src);
    expect(logo.transparent).toBe(true);
  });
});

describe('pinned Console free model ids', () => {
  test('matches the documented free list (10 ids, incl. the unsuffixed big-pickle)', () => {
    // Reference: https://opencode.ai/v2/docs/console/models/#free-models
    expect(OPENCODE_CONSOLE_FREE_MODEL_IDS).toHaveLength(10);
    expect(OPENCODE_CONSOLE_FREE_MODEL_IDS).toContain('big-pickle');
    expect(OPENCODE_CONSOLE_FREE_MODEL_IDS).toContain('glm-5.3-flash-free');
    expect(OPENCODE_CONSOLE_FREE_MODEL_IDS).toContain('nemotron-3-ultra-free');
    expect(OPENCODE_CONSOLE_FREE_MODEL_IDS.every((id) => id === id.toLowerCase())).toBe(true);
  });
});
