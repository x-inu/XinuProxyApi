import { describe, expect, it } from 'bun:test';
import { OAUTH_PROVIDER_DEFS } from '../src/features/providers/descriptors';

describe('Providers Overview Grid & OAuth Unification', () => {
  it('defines the complete standard list of OAuth providers', () => {
    const ids = OAUTH_PROVIDER_DEFS.map((p) => p.id);
    expect(ids).toContain('antigravity');
    expect(ids).toContain('codex');
    expect(ids).toContain('claude');
    expect(ids).toContain('kimi');
    expect(ids).toContain('meta');
    expect(ids).toContain('xai');
    expect(ids).toContain('devin');
    expect(ids).toContain('vertex');
  });

  it('contains valid display names for each OAuth provider', () => {
    for (const def of OAUTH_PROVIDER_DEFS) {
      expect(def.name).toBeTruthy();
      expect(typeof def.name).toBe('string');
      expect(def.name.length).toBeGreaterThan(3);
    }
  });
});
