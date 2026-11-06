import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { IconPlus, IconSearch, IconSlidersHorizontal, IconX } from '@/components/ui/icons';
import { Button } from '@/components/ui/Button';
import { ProviderCard, type ProviderStats } from './ProviderCard';
import { OAUTH_PROVIDER_DEFS } from '../descriptors';
import type { ProviderBrand, ProviderGroup, ProviderResource } from '../types';
import type { AuthFileItem } from '@/types/authFile';
import styles from './ProvidersOverviewGrid.module.scss';

interface Props {
  groups: ProviderGroup[];
  authFiles: AuthFileItem[];
  onSelectConfigProvider: (brand: ProviderBrand, resource?: ProviderResource) => void;
  onAddCustomProvider: () => void;
  onToggleConfigProvider: (brand: ProviderBrand, resource: ProviderResource, active: boolean) => void;
  onSelectOAuthProvider: (providerId: string, providerName: string) => void;
  onToggleOAuthProvider: (providerId: string, active: boolean) => void;
}

type StatusFilter = 'all' | 'connected' | 'error' | 'disabled' | 'not_connected';

export function ProvidersOverviewGrid({
  groups,
  authFiles,
  onSelectConfigProvider,
  onAddCustomProvider,
  onToggleConfigProvider,
  onSelectOAuthProvider,
  onToggleOAuthProvider,
}: Props) {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  const matchesSearch = useCallback(
    (text: string) => {
      const q = searchQuery.trim().toLowerCase();
      if (!q) return true;
      return text.toLowerCase().includes(q);
    },
    [searchQuery]
  );

  const matchesStatus = useCallback(
    (stats: ProviderStats, noAuth = false) => {
      switch (statusFilter) {
        case 'connected':
          return stats.connected > 0 || noAuth;
        case 'error':
          return stats.errorCount > 0;
        case 'disabled':
          return stats.allDisabled;
        case 'not_connected':
          return stats.total === 0 && !noAuth;
        default:
          return true;
      }
    },
    [statusFilter]
  );

  // 1. Custom Providers (OpenAI-compatible endpoints)
  const customGroup = useMemo(
    () => groups.find((g) => g.id === 'openaiCompatibility'),
    [groups]
  );
  const customProviders = useMemo(() => {
    if (!customGroup) return [];
    return customGroup.resources
      .map((r) => {
        const keyCount = r.apiKeyEntryCount || (r.apiKey ? 1 : 0);
        const stats: ProviderStats = {
          total: keyCount,
          connected: r.disabled ? 0 : keyCount,
          disabledCount: r.disabled ? Math.max(keyCount, 1) : 0,
          errorCount: 0,
          allDisabled: r.disabled,
        };
        const displayName = r.name || r.identifier || 'Custom Provider';
        return {
          resource: r,
          displayName,
          stats,
        };
      })
      .filter(
        (item) =>
          matchesSearch(item.displayName) && matchesStatus(item.stats)
      );
  }, [customGroup, matchesSearch, matchesStatus]);

  // 2. OAuth Providers
  const oauthCards = useMemo(() => {
    return OAUTH_PROVIDER_DEFS.map((def) => {
      const files = authFiles.filter((f) => {
        const normProv = (f.provider || f.type || '').toLowerCase();
        if (def.id === 'claude') {
          return normProv === 'claude' || normProv === 'anthropic';
        }
        return normProv === def.id.toLowerCase();
      });

      const total = files.length;
      const connected = files.filter((f) => !f.disabled && !f.unavailable).length;
      const errorCount = files.filter(
        (f) => Boolean(f.unavailable || (f.cooldownSnapshot?.records && f.cooldownSnapshot.records.length > 0))
      ).length;
      const disabledCount = files.filter((f) => f.disabled).length;
      const allDisabled = total > 0 && files.every((f) => f.disabled);

      const stats: ProviderStats = {
        total,
        connected,
        errorCount,
        disabledCount,
        allDisabled,
      };

      return {
        ...def,
        stats,
      };
    }).filter(
      (p) =>
        (matchesSearch(p.name) || matchesSearch(p.id)) &&
        matchesStatus(p.stats)
    );
  }, [authFiles, matchesSearch, matchesStatus]);

  // 3. Free Tier Providers (OpenCode Free)
  const freeGroup = useMemo(
    () => groups.find((g) => g.id === 'opencodeFree'),
    [groups]
  );
  const freeTierProviders = useMemo(() => {
    if (!freeGroup || freeGroup.resources.length === 0) return [];
    const r = freeGroup.resources[0];
    const slotCount = r.apiKeyEntryCount || 1;
    const stats: ProviderStats = {
      total: slotCount,
      connected: r.disabled ? 0 : slotCount,
      disabledCount: r.disabled ? slotCount : 0,
      errorCount: 0,
      allDisabled: r.disabled,
    };

    const displayName = t('providersPage.providerNames.opencodeFree', {
      defaultValue: 'OpenCode (free)',
    });

    if (!matchesSearch(displayName) && !matchesSearch('opencodeFree')) return [];
    if (!matchesStatus(stats, true)) return [];

    return [
      {
        resource: r,
        displayName,
        stats,
      },
    ];
  }, [freeGroup, matchesSearch, matchesStatus, t]);

  // 4. API Key Providers (Gemini, Claude, OpenAI, etc.) — always 1 grouped card per brand
  const apikeyProviders = useMemo(() => {
    return groups
      .filter(
        (g) =>
          g.id !== 'openaiCompatibility' &&
          g.id !== 'opencodeFree'
      )
      .map((g) => {
        const displayName = t(`providersPage.providerNames.${g.id}`, {
          defaultValue: g.id,
        });

        const total = g.resources.reduce(
          (sum, r) => sum + (r.apiKeyEntryCount || (r.apiKey ? 1 : 0)),
          0
        );
        const connected = g.resources.reduce(
          (sum, r) => sum + (r.disabled ? 0 : r.apiKeyEntryCount || (r.apiKey ? 1 : 0)),
          0
        );
        const disabledCount = g.resources.reduce(
          (sum, r) =>
            sum + (r.disabled ? Math.max(r.apiKeyEntryCount || (r.apiKey ? 1 : 0), 1) : 0),
          0
        );
        const allDisabled = g.resources.length > 0 && g.resources.every((r) => r.disabled);

        const stats: ProviderStats = {
          total,
          connected,
          disabledCount,
          errorCount: 0,
          allDisabled,
        };

        return {
          brand: g.id,
          displayName,
          stats,
          firstResource: g.resources[0],
        };
      })
      .filter(
        (p) =>
          (matchesSearch(p.displayName) || matchesSearch(p.brand)) &&
          matchesStatus(p.stats)
      );
  }, [groups, matchesSearch, matchesStatus, t]);

  return (
    <div className={styles.container}>
      {/* Search / Filter Bar */}
      <div className={styles.filterBar}>
        <div className={styles.searchWrapper}>
          <IconSearch size={15} className={styles.searchIcon} />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t('providersPage.search_placeholder', {
              defaultValue: 'Search providers...',
            })}
            className={styles.searchInput}
          />
          {searchQuery ? (
            <button
              type="button"
              onClick={() => setSearchQuery('')}
              className={styles.searchClear}
              aria-label="Clear search"
            >
              <IconX size={14} />
            </button>
          ) : null}
        </div>

        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
          className={styles.statusSelect}
        >
          <option value="all">All Status</option>
          <option value="connected">Connected</option>
          <option value="error">Error</option>
          <option value="disabled">Disabled</option>
          <option value="not_connected">Not Connected</option>
        </select>
      </div>

      {/* 1. Custom Providers */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>
            Custom Providers (OpenAI, Anthropic, Custom)
          </h2>
          <Button size="sm" onClick={onAddCustomProvider}>
            <IconPlus size={14} style={{ marginRight: 4 }} />
            Add Custom Provider
          </Button>
        </div>

        {customProviders.length === 0 ? (
          <div className={styles.emptyBox}>
            <IconSlidersHorizontal size={18} />
            <span>
              No custom providers — use “Add Custom Provider” to register an OpenAI, Anthropic, or custom endpoint
            </span>
          </div>
        ) : (
          <div className={styles.grid}>
            {customProviders.map((cp) => (
              <ProviderCard
                key={cp.resource.identifier}
                id="openaiCompatibility"
                name={cp.displayName}
                stats={cp.stats}
                onClick={() => onSelectConfigProvider('openaiCompatibility', cp.resource)}
                onToggleAll={(active) =>
                  onToggleConfigProvider('openaiCompatibility', cp.resource, active)
                }
              />
            ))}
          </div>
        )}
      </div>

      {/* 2. OAuth Providers */}
      {oauthCards.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>OAuth Providers</h2>
          </div>

          <div className={styles.grid}>
            {oauthCards.map((p) => (
              <ProviderCard
                key={p.id}
                id={p.id}
                name={p.name}
                stats={p.stats}
                onClick={() => onSelectOAuthProvider(p.id, p.name)}
                onToggleAll={(active) => onToggleOAuthProvider(p.id, active)}
              />
            ))}
          </div>
        </div>
      )}

      {/* 3. Free Tier Providers */}
      {freeTierProviders.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>Free Tier Providers</h2>
          </div>

          <div className={styles.grid}>
            {freeTierProviders.map((ftp) => (
              <ProviderCard
                key="opencodeFree"
                id="opencodeFree"
                name={ftp.displayName}
                stats={ftp.stats}
                noAuth
                onClick={() => onSelectConfigProvider('opencodeFree', ftp.resource)}
                onToggleAll={(active) =>
                  onToggleConfigProvider('opencodeFree', ftp.resource, active)
                }
              />
            ))}
          </div>
        </div>
      )}

      {/* 4. API Key Providers */}
      {apikeyProviders.length > 0 && (
        <div className={styles.section}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>API Key Providers</h2>
          </div>

          <div className={styles.grid}>
            {apikeyProviders.map((p) => (
              <ProviderCard
                key={p.brand}
                id={p.brand}
                name={p.displayName}
                stats={p.stats}
                onClick={() => onSelectConfigProvider(p.brand, p.firstResource)}
                onToggleAll={
                  p.firstResource
                    ? (active) => {
                        onToggleConfigProvider(p.brand, p.firstResource!, active);
                      }
                    : undefined
                }
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
