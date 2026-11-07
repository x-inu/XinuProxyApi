import { describe, expect, test } from 'bun:test';
import { normalizeProviderKeyConfig } from '../src/services/api/transformers';
import { groupConfigCredentials } from '../src/features/models/modelsCatalog';
import { claudeToResource } from '../src/features/providers/adapters';
import {
  buildOpenAIConfig,
  buildProviderKeyConfig,
} from '../src/features/providers/useProviderWorkbench';
import type { Config, ProviderKeyConfig } from '../src/types';

describe('claude multi-key (api-key-entries) normalization', () => {
  test('keeps an entry with empty api-key when api-key-entries exists', () => {
    const config = normalizeProviderKeyConfig({
      'api-key': '',
      'base-url': 'https://gateway.example.com',
      'api-key-entries': [
        { 'api-key': ' key-a ' },
        { 'api-key': 'key-b', 'proxy-url': 'http://proxy:8080', weight: 3 },
        { 'api-key': '' },
      ],
    });

    expect(config).not.toBeNull();
    expect(config?.apiKey).toBe('');
    expect(config?.baseUrl).toBe('https://gateway.example.com');
    expect(config?.apiKeyEntries).toHaveLength(2);
    expect(config?.apiKeyEntries?.[0]?.apiKey).toBe('key-a');
    expect(config?.apiKeyEntries?.[1]?.apiKey).toBe('key-b');
    expect(config?.apiKeyEntries?.[1]?.proxyUrl).toBe('http://proxy:8080');
    expect(config?.apiKeyEntries?.[1]?.weight).toBe(3);
  });

  test('still drops entries without api key and without entries', () => {
    expect(normalizeProviderKeyConfig({ 'api-key': '' })).toBeNull();
    expect(
      normalizeProviderKeyConfig({ 'api-key': '', 'api-key-entries': [{ 'api-key': '' }] })
    ).toBeNull();
  });

  test('preserves emptied Base URL group when base-url or prefix is configured', () => {
    const config = normalizeProviderKeyConfig({
      'api-key': '',
      prefix: 'jdw',
      'base-url': 'https://api.justwoker.icu',
      'api-key-entries': [],
    });
    expect(config).not.toBeNull();
    expect(config?.apiKey).toBe('');
    expect(config?.prefix).toBe('jdw');
    expect(config?.baseUrl).toBe('https://api.justwoker.icu');
    expect(config?.apiKeyEntries).toBeUndefined();

    const resource = claudeToResource(config!, 0);
    expect(resource.apiKeyEntryCount).toBe(0);
    expect(resource.identifier).toBe('jdw · https://api.justwoker.icu');
  });

  test('omits apiKeyEntries for plain single-key credentials', () => {
    const config = normalizeProviderKeyConfig({ 'api-key': 'solo' });
    expect(config?.apiKeyEntries).toBeUndefined();
  });

  test('buildProviderKeyConfig empties claude apiKeyEntries cleanly without resurrecting old keys', () => {
    const existing: ProviderKeyConfig = {
      apiKey: '',
      prefix: 'jdw',
      baseUrl: 'https://api.justwoker.icu',
      apiKeyEntries: [{ apiKey: 'old-key-1' }, { apiKey: 'old-key-2' }],
    };

    const emptied = buildProviderKeyConfig(
      'claude',
      {
        apiKey: '',
        name: '',
        baseUrl: 'https://api.justwoker.icu',
        proxyUrl: '',
        prefix: 'jdw',
        disabled: true,
        disableCooling: true,
        models: [],
        headers: [],
        excludedModelsText: '',
        apiKeyEntries: [{ apiKey: '', proxyUrl: '' }],
      },
      existing
    ) as ProviderKeyConfig;

    expect(emptied.apiKey).toBe('');
    expect(emptied.apiKeyEntries).toEqual([]);
  });

  test('buildOpenAIConfig empties apiKeyEntries cleanly without resurrecting old keys', () => {
    const emptied = buildOpenAIConfig(
      {
        apiKey: '',
        name: 'Nous',
        baseUrl: 'https://inference-api.nousresearch.com/v1',
        proxyUrl: '',
        prefix: 'nous',
        disabled: true,
        disableCooling: false,
        models: [],
        headers: [],
        excludedModelsText: '',
        apiKeyEntries: [{ apiKey: '', proxyUrl: '' }],
      },
      {
        name: 'Nous',
        baseUrl: 'https://inference-api.nousresearch.com/v1',
        apiKeyEntries: [{ apiKey: 'old-nous-key' }],
      }
    );

    expect(emptied.apiKeyEntries).toEqual([]);
  });
});

describe('claude multi-key model cards', () => {
  test('builds one card per claude entry with first-key discovery', () => {
    const config: Config = {
      claudeApiKeys: [
        {
          apiKey: '',
          baseUrl: 'https://gateway.example.com',
          apiKeyEntries: [
            { apiKey: 'key-a' },
            { apiKey: 'key-b' },
            { apiKey: 'key-c' },
          ],
        },
        { apiKey: 'legacy-key', baseUrl: 'https://legacy.example.com' },
      ],
    } as Config;

    const groups = groupConfigCredentials(config);
    const claude = groups.find((group) => group.id === 'claude');
    expect(claude?.cards).toHaveLength(2);

    const multi = claude?.cards[0];
    expect(multi?.id).toBe('key:claude:0');
    expect(multi?.title).toContain('ke');
    expect(multi?.title).toContain('+2');
    expect(multi?.discovery).not.toBeNull();
    expect(multi?.discovery?.apiKey).toBe('key-a');
    expect(multi?.mutation.type).toBe('config-key');
    expect(multi?.mutation).toMatchObject({ type: 'config-key', apiKey: null });
    expect(claude?.cards[1]?.discovery?.apiKey).toBe('legacy-key');
  });
});
