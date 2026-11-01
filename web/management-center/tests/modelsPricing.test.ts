import { describe, expect, test } from 'bun:test';
import { normalizeModelList } from '../src/utils/models';
import { applyFreeOnlyFilter } from '../src/features/models/modelsCatalog';

describe('normalizeModelList pricing v2 (cost block)', () => {
  test('OpenCode cost block with input and output price 0 reports a free model', () => {
    const payload = {
      data: [
        {
          id: 'big-pickle',
          cost: { text: { input: { price: 0, cache: { read: 0, write: 0 } }, output: { price: 0 } } },
        },
      ],
    };
    const models = normalizeModelList(payload);
    expect(models).toHaveLength(1);
    expect(models[0].name).toBe('big-pickle');
    expect(models[0].isFree).toBe(true);
  });

  test('non-zero prices report a paid model, overrides included', () => {
    const payload = {
      data: [
        {
          id: 'gpt-5.5',
          cost: {
            text: {
              input: {
                price: 5,
                cache: { read: 0.5 },
                overrides: [{ when: { context: { gt: 272000 } }, price: 10 }],
              },
              output: { price: 30 },
            },
          },
        },
        {
          id: 'glm-5.3-flash-free',
          cost: { text: { input: { price: 0, cache: { read: 0 } }, output: { price: 0 } } },
        },
      ],
    };
    const models = normalizeModelList(payload);
    expect(models.find((model) => model.name === 'gpt-5.5')?.isFree).toBe(false);
    expect(models.find((model) => model.name === 'glm-5.3-flash-free')?.isFree).toBe(true);
  });

  test('missing or partial cost stays unknown so the suffix fallback decides', () => {
    const payload = {
      data: [
        { id: 'beam-free', name: 'Beam Free' },
        { id: 'claude-opus-5', cost: { text: { input: { price: 3 } } } },
        { id: 'kimi-k2.7', cost: {} },
      ],
    };
    const models = normalizeModelList(payload);
    expect(models.map((model) => model.isFree)).toEqual([undefined, undefined, undefined]);

    // Without a price block, models without pricing stay visible in a plain
    // free-only filter, and the OpenCode suffix helper narrows its own ids.
    expect(applyFreeOnlyFilter(models, true)).toHaveLength(3);
  });

  test('legacy pricing.prompt shape still wins when present', () => {
    const payload = {
      data: [
        { id: 'legacy-free', pricing: { prompt: 0, completion: 0 } },
        { id: 'legacy-paid', pricing: { prompt: '2.5', completion: '10' } },
      ],
    };
    const models = normalizeModelList(payload);
    expect(models.find((model) => model.name === 'legacy-free')?.isFree).toBe(true);
    expect(models.find((model) => model.name === 'legacy-paid')?.isFree).toBe(false);
  });
});
