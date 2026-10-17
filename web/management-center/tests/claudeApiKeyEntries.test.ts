import { describe, expect, test } from 'bun:test';
import { normalizeProviderKeyConfig } from '../src/services/api/transformers';
import { groupConfigCredentials } from '../src/features/models/modelsCatalog';
import type { Config } from '../src/types';

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

  test('omits apiKeyEntries for plain single-key credentials', () => {
    const config = normalizeProviderKeyConfig({ 'api-key': 'solo' });
    expect(config?.apiKeyEntries).toBeUndefined();
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
