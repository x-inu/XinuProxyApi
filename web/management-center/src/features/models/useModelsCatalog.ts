import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { authFilesApi, providersApi } from '@/services/api';
import {
  patchAuthFileExcludedModels,
  patchConfigKeyExcludedModels,
  patchOpenAICompatModels,
} from '@/services/api/modelAvailability';
import { getErrorMessage, isRecord } from '@/utils/helpers';
import type { ModelAlias } from '@/types';
import type { ModelInfo } from '@/utils/models';
import type { AuthFileItem } from '@/types/authFile';
import { useAuthStore, useConfigStore, useNotificationStore } from '@/stores';
import { PROVIDER_BRAND_ORDER } from '../providers/descriptors';
import {
  fetchUpstreamModels,
  isUpstreamDiscoveryBrand,
  getDiscoveryErrorMessage,
} from './upstreamDiscovery';
import {
  applyFreeOnlyFilter,
  buildModelRows,
  groupConfigCredentials,
  toggleExactExclusion,
  type CredentialCard,
  type ModelRow,
  type ProviderCardGroup,
} from './modelsCatalog';
import { normalizeExcludedRules } from '@/components/excludedModels';

const CACHE_STORAGE_KEY = 'xinu.models.catalog.v1';
const CACHE_TTL_MS = 10 * 60 * 1000;
const FETCH_ALL_CONCURRENCY = 3;
const FILE_RULES_CONCURRENCY = 4;

type ExcludeFieldKey = 'excluded_models' | 'excluded-models';

export interface CatalogEntryState {
  loading: boolean;
  error: string | null;
  models: ModelInfo[];
  fetchedAt: string | null;
}

export interface FileRuleState {
  loading: boolean;
  error: string | null;
  rules: string[] | null;
  excludeFieldKey: ExcludeFieldKey;
  /** Effective models fallback for providers without a static catalog. */
  catalogNames: string[] | null;
}

export interface ChannelCatalogState {
  loading: boolean;
  error: string | null;
  names: string[];
}

export interface OAuthGroup {
  providerKey: string;
  files: AuthFileItem[];
  cards: CredentialCard[];
}

const emptyCatalogEntry: CatalogEntryState = {
  loading: false,
  error: null,
  models: [],
  fetchedAt: null,
};

interface CatalogCacheShape {
  models: ModelInfo[];
  at: number;
}

const readCatalogCache = (): Record<string, CatalogCacheShape> => {
  try {
    const raw = sessionStorage.getItem(CACHE_STORAGE_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    return isRecord(parsed) ? (parsed as Record<string, CatalogCacheShape>) : {};
  } catch {
    return {};
  }
};

const writeCatalogCache = (key: string, models: ModelInfo[]) => {
  try {
    const cache = readCatalogCache();
    cache[key] = { models, at: Date.now() };
    sessionStorage.setItem(CACHE_STORAGE_KEY, JSON.stringify(cache));
  } catch {
    // storage may be unavailable; the discovery cache is best-effort only
  }
};

const cacheIsFresh = (entry?: CatalogCacheShape): boolean =>
  Boolean(entry && Date.now() - entry.at < CACHE_TTL_MS);

const normalizeProviderKey = (value: string | undefined | null): string =>
  (value ?? '').trim().toLowerCase() || 'unknown';

/** Reads per-file excluded models from a downloaded auth file JSON body. */
export const parseAuthFileExcludedRules = (
  text: string
): { rules: string[]; excludeFieldKey: ExcludeFieldKey } => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { rules: [], excludeFieldKey: 'excluded_models' };
  }
  const obj = isRecord(parsed) ? parsed : {};
  const snake = obj['excluded_models'];
  const kebab = obj['excluded-models'];
  const rules = Array.isArray(snake) ? snake : Array.isArray(kebab) ? kebab : [];
  return {
    rules: normalizeExcludedRules(rules.map((value) => String(value))),
    excludeFieldKey: Array.isArray(snake) || snake === null ? 'excluded_models' : 'excluded-models',
  };
};

export function useModelsCatalog() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((s) => s.connectionStatus);
  const config = useConfigStore((s) => s.config);
  const fetchConfig = useConfigStore((s) => s.fetchConfig);
  const updateConfigValue = useConfigStore((s) => s.updateConfigValue);
  const isCacheValid = useConfigStore((s) => s.isCacheValid);
  const showNotification = useNotificationStore((s) => s.showNotification);

  const connected = connectionStatus === 'connected';

  const [isPending, setIsPending] = useState(() => !isCacheValid());
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [catalogs, setCatalogs] = useState<Record<string, CatalogEntryState>>(() => {
    const entries: Record<string, CatalogEntryState> = {};
    for (const [key, value] of Object.entries(readCatalogCache())) {
      if (!cacheIsFresh(value) || !Array.isArray(value.models)) continue;
      entries[key] = {
        loading: false,
        error: null,
        models: value.models.filter((model) => typeof model?.name === 'string'),
        fetchedAt: value.at ? new Date(value.at).toISOString() : null,
      };
    }
    return entries;
  });
  const [oauthFiles, setOauthFiles] = useState<AuthFileItem[] | null>(null);
  const [oauthError, setOauthError] = useState<string | null>(null);
  const [globalRulesByProvider, setGlobalRulesByProvider] = useState<Record<string, string[]>>({});
  /** Per-card "Free only" fetch filter; undefined = default for its group. */
  const [freeOnlyByCard, setFreeOnlyByCard] = useState<Record<string, boolean>>({});
  const [globalRulesLoaded, setGlobalRulesLoaded] = useState(false);
  const [channelCatalogs, setChannelCatalogs] = useState<Record<string, ChannelCatalogState>>({});
  const [fileStates, setFileStates] = useState<Record<string, FileRuleState>>({});
  const [fetchAllState, setFetchAllState] = useState<{ active: boolean; done: number; total: number }>({
    active: false,
    done: 0,
    total: 0,
  });
  const [togglingKeys, setTogglingKeys] = useState<Set<string>>(new Set());

  const hasFetchedRef = useRef(false);
  const loadedOAuthRef = useRef<Set<string>>(new Set());
  const catalogsRef = useRef(catalogs);
  catalogsRef.current = catalogs;

  /* ------------------------------ data load ----------------------------- */

  const refetchConfig = useCallback(async () => {
    const [configResult, vertexResult, openaiResult] = await Promise.allSettled([
      fetchConfig(true),
      providersApi.getVertexConfigs(),
      providersApi.getOpenAIProviders(),
    ]);
    if (configResult.status === 'fulfilled') {
      if (vertexResult.status === 'fulfilled') {
        updateConfigValue('vertex-api-key', vertexResult.value || []);
      }
      if (openaiResult.status === 'fulfilled') {
        updateConfigValue('openai-compatibility', openaiResult.value || []);
      }
    }
  }, [fetchConfig, updateConfigValue]);

  const refetch = useCallback(async () => {
    setIsPending(true);
    setErrorMessage(null);
    try {
      await refetchConfig();
      const [filesResult, rulesResult] = await Promise.allSettled([
        authFilesApi.list(),
        authFilesApi.getOauthExcludedModels(),
      ]);
      if (filesResult.status === 'fulfilled') {
        setOauthFiles(filesResult.value.files ?? []);
        setOauthError(null);
      } else {
        setOauthError(getErrorMessage(filesResult.reason));
      }
      if (rulesResult.status === 'fulfilled') {
        setGlobalRulesByProvider(rulesResult.value ?? {});
      }
      setGlobalRulesLoaded(true);
      loadedOAuthRef.current = new Set();
    } catch (err) {
      setErrorMessage(getErrorMessage(err) || 'Failed to load models');
    } finally {
      setIsPending(false);
    }
  }, [refetchConfig]);

  useEffect(() => {
    if (hasFetchedRef.current) return;
    if (!connected) return;
    hasFetchedRef.current = true;
    void refetch();
  }, [connected, refetch]);

  /* ------------------------- provider cards (config) -------------------- */

  const groups = useMemo<ProviderCardGroup[]>(() => {
    if (!config) return [];
    const order = (brand: string): number => {
      const index = (PROVIDER_BRAND_ORDER as readonly string[]).indexOf(brand);
      return index === -1 ? PROVIDER_BRAND_ORDER.length : index;
    };
    return groupConfigCredentials(config)
      .filter((group) => group.cards.length > 0)
      .sort((left, right) => order(left.brand) - order(right.brand));
  }, [config]);

  const providerCards = useMemo<CredentialCard[]>(
    () => groups.flatMap((group) => group.cards),
    [groups]
  );

  /* ----------------------------- oauth groups ---------------------------- */

  const oauthGroups = useMemo<OAuthGroup[]>(() => {
    if (!oauthFiles) return [];
    const byProvider = new Map<string, AuthFileItem[]>();
    for (const file of oauthFiles) {
      const providerKey = normalizeProviderKey(file.provider ?? file.type);
      const list = byProvider.get(providerKey) ?? [];
      list.push(file);
      byProvider.set(providerKey, list);
    }
    return Array.from(byProvider.entries())
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([providerKey, files]) => {
        // One merged card per OAuth provider: the group's model exclusions live
        // in the global oauth-excluded-models layer, so account names must not
        // surface in operator-facing text on this card.
        const allDisabled = files.length > 0 && files.every((file) => file.disabled === true);
        const mergedCard: CredentialCard = {
          id: `auth-group:${providerKey}`,
          kind: 'oauth-group',
          groupKey: `oauth:${providerKey}`,
          brand: 'openaiCompatibility',
          title: providerKey,
          subtitle: t('models_page.oauth_group_subtitle', { count: files.length }),
          lockDisabled: allDisabled,
          disableAll: false,
          configuredIds: [],
          excludedRules: [],
          aliases: {},
          discovery: null,
          mutation: { type: 'oauth-group', providerKey },
        };
        return { providerKey, files, cards: [mergedCard] };
      });
  }, [oauthFiles, t]);

  const oauthCards = useMemo<CredentialCard[]>(
    () => oauthGroups.flatMap((group) => group.cards),
    [oauthGroups]
  );

  /* ------------------------------ catalogs ------------------------------- */

  /** Fetches (or re-fetches) the upstream catalog for one credential card. */
  const fetchCatalog = useCallback(
    async (card: CredentialCard, force = false): Promise<'ok' | 'error' | 'skipped'> => {
      if (!card.discovery || !isUpstreamDiscoveryBrand(card.discovery.brand)) {
        return 'skipped';
      }
      const existing = catalogsRef.current[card.id];
      if (
        !force &&
        existing &&
        (existing.loading || (existing.models.length > 0 && !existing.error))
      ) {
        return 'skipped';
      }
      setCatalogs((prev) => ({
        ...prev,
        [card.id]: { ...emptyCatalogEntry, loading: true },
      }));
      try {
        const models = await fetchUpstreamModels(card.discovery);
        setCatalogs((prev) => ({
          ...prev,
          [card.id]: {
            loading: false,
            error: null,
            models,
            fetchedAt: new Date().toISOString(),
          },
        }));
        writeCatalogCache(card.id, models);
        return 'ok';
      } catch (err) {
        setCatalogs((prev) => ({
          ...prev,
          [card.id]: {
            loading: false,
            error: getDiscoveryErrorMessage(err),
            models: [],
            fetchedAt: null,
          },
        }));
        return 'error';
      }
    },
    []
  );

  const fetchAllCatalogs = useCallback(
    async (targetCards: readonly CredentialCard[]) => {
      const targets = targetCards.filter(
        (card) => card.discovery && isUpstreamDiscoveryBrand(card.discovery.brand)
      );
      if (!targets.length) {
        showNotification(t('models_page.fetch_all.none'), 'info');
        return;
      }
      setFetchAllState({ active: true, done: 0, total: targets.length });
      let done = 0;
      let failures = 0;
      const queue = [...targets];
      const worker = async () => {
        while (queue.length > 0) {
          const card = queue.shift();
          if (!card) return;
          const result = await fetchCatalog(card, true);
          if (result === 'error') failures += 1;
          done += 1;
          setFetchAllState((prev) => ({ ...prev, done }));
        }
      };
      try {
        await Promise.all(
          Array.from({ length: Math.min(FETCH_ALL_CONCURRENCY, targets.length) }, () => worker())
        );
      } finally {
        setFetchAllState({ active: false, done: targets.length, total: targets.length });
      }
      showNotification(
        failures > 0
          ? t('models_page.fetch_all.done_with_errors', { total: targets.length, failed: failures })
          : t('models_page.fetch_all.done', { total: targets.length }),
        failures > 0 ? 'warning' : 'success'
      );
    },
    [fetchCatalog, showNotification, t]
  );

  /* --------------------------- oauth file rules -------------------------- */

  const loadChannelCatalog = useCallback(async (group: OAuthGroup) => {
    const providerKey = group.providerKey;
    let useChannelCatalog = false;
    setChannelCatalogs((prev) => ({
      ...prev,
      [providerKey]: { loading: true, error: null, names: prev[providerKey]?.names ?? [] },
    }));
    try {
      const models = await authFilesApi.getModelDefinitions(providerKey);
      const names = models.map((model) => model.id).filter(Boolean);
      setChannelCatalogs((prev) => ({
        ...prev,
        [providerKey]: { loading: false, error: null, names },
      }));
      useChannelCatalog = true;
    } catch {
      setChannelCatalogs((prev) => ({
        ...prev,
        [providerKey]: {
          loading: false,
          error: 'unavailable',
          names: prev[providerKey]?.names ?? [],
        },
      }));
    }
    if (useChannelCatalog) return;
    // No static catalog for this provider — read effective models per file
    // as the candidate list so toggles still have a full catalog to work on.
    await Promise.all(
      group.files.map(async (file) => {
        try {
          const models = await authFilesApi.getModelsForAuthFile(file.name);
          setFileStates((prev) => ({
            ...prev,
            [file.name]: {
              ...(prev[file.name] ?? {
                loading: false,
                error: null,
                rules: null,
                excludeFieldKey: 'excluded_models' as ExcludeFieldKey,
              }),
              catalogNames: models.map((model) => model.id).filter(Boolean),
            },
          }));
        } catch {
          // Leave the fallback absent; rule-only rows still render.
        }
      })
    );
  }, []);

  const loadFileRules = useCallback(async (fileNames: readonly string[]) => {
    const queue = fileNames.filter(Boolean);
    setFileStates((prev) => {
      const next = { ...prev };
      for (const fileName of queue) {
        if (prev[fileName]?.loading) continue;
        if (Array.isArray(prev[fileName]?.rules)) continue;
        next[fileName] = {
          loading: true,
          error: null,
          rules: null,
          excludeFieldKey: 'excluded_models',
          catalogNames: prev[fileName]?.catalogNames ?? null,
        };
      }
      return next;
    });
    const pending = [...queue];
    const worker = async () => {
      while (pending.length > 0) {
        const fileName = pending.shift();
        if (!fileName) return;
        try {
          const text = await authFilesApi.downloadText(fileName);
          const { rules, excludeFieldKey } = parseAuthFileExcludedRules(text);
          setFileStates((prev) => ({
            ...prev,
            [fileName]: {
              loading: false,
              error: null,
              rules,
              excludeFieldKey,
              catalogNames: prev[fileName]?.catalogNames ?? null,
            },
          }));
        } catch (err) {
          setFileStates((prev) => ({
            ...prev,
            [fileName]: {
              loading: false,
              error: getErrorMessage(err) || 'Failed to read auth file',
              rules: null,
              excludeFieldKey: 'excluded_models',
              catalogNames: prev[fileName]?.catalogNames ?? null,
            },
          }));
        }
      }
    };
    await Promise.all(
      Array.from({ length: Math.min(FILE_RULES_CONCURRENCY, Math.max(1, pending.length)) }, () =>
        worker()
      )
    );
  }, []);

  /** Loads the static catalog and per-file rules once per OAuth provider. */
  const ensureOAuthProviderData = useCallback(
    async (group: OAuthGroup) => {
      if (loadedOAuthRef.current.has(group.providerKey)) return;
      loadedOAuthRef.current.add(group.providerKey);
      await Promise.all([loadChannelCatalog(group), loadFileRules(group.files.map((f) => f.name))]);
    },
    [loadChannelCatalog, loadFileRules]
  );

  const refreshOAuthGroup = useCallback(
    async (group: OAuthGroup) => {
      loadedOAuthRef.current.delete(group.providerKey);
      setChannelCatalogs((prev) => {
        const next = { ...prev };
        delete next[group.providerKey];
        return next;
      });
      setFileStates((prev) => {
        const next = { ...prev };
        for (const file of group.files) delete next[file.name];
        return next;
      });
      await ensureOAuthProviderData(group);
    },
    [ensureOAuthProviderData]
  );

  /* ------------------------------- toggles ------------------------------- */

  const toggleModel = useCallback(
    async (card: CredentialCard, row: ModelRow, nextEnabled: boolean) => {
      if (row.locked) return;
      const key = `${card.id}::${row.id}`;
      if (togglingKeys.has(key)) return;
      setTogglingKeys((prev) => {
        const next = new Set(prev);
        next.add(key);
        return next;
      });
      try {
        switch (card.mutation.type) {
          case 'config-key': {
            const nextRules = toggleExactExclusion(card.excludedRules, row.id, !nextEnabled);
            await patchConfigKeyExcludedModels({
              brand: card.mutation.keyBrand,
              index: card.mutation.index,
              apiKey: card.mutation.apiKey,
              excludedModels: nextRules,
            });
            await refetchConfig();
            break;
          }
          case 'openai-compat': {
            const matches = (name: string) => name.toLowerCase() === row.id.toLowerCase();
            const nextModels: ModelAlias[] = nextEnabled
              ? card.mutation.models.some((model) => matches(model.name))
                ? card.mutation.models
                : [...card.mutation.models, { name: row.id }]
              : card.mutation.models.filter((model) => !matches(model.name));
            await patchOpenAICompatModels({
              name: card.mutation.name,
              index: card.mutation.index,
              models: nextModels,
            });
            await refetchConfig();
            break;
          }
          case 'oauth-file': {
            const mutation = card.mutation;
            const fileName = mutation.type === 'oauth-file' ? mutation.fileName : '';
            const state = fileStates[fileName];
            const currentRules = state?.rules ?? [];
            const nextRules = toggleExactExclusion(currentRules, row.id, !nextEnabled);
            await patchAuthFileExcludedModels({
              name: fileName,
              excludeFieldKey: state?.excludeFieldKey ?? 'excluded_models',
              excludedModels: nextRules,
            });
            setFileStates((prev) => ({
              ...prev,
              [fileName]: {
                loading: false,
                error: null,
                rules: nextRules,
                excludeFieldKey: state?.excludeFieldKey ?? 'excluded_models',
                catalogNames: state?.catalogNames ?? null,
              },
            }));
            break;
          }
          case 'oauth-group': {
            // Group toggle = one write to the global rules layer, affecting
            // every account of the provider at once.
            const providerKey = card.mutation.providerKey;
            const currentRules = globalRulesByProvider[providerKey] ?? [];
            const nextRules = toggleExactExclusion(currentRules, row.id, !nextEnabled);
            await authFilesApi.saveOauthExcludedModels(providerKey, nextRules);
            setGlobalRulesByProvider((prev) => ({
              ...prev,
              [providerKey]: nextRules,
            }));
            showNotification(
              t('models_page.toast.group_rules_updated', { count: nextRules.length }),
              'success'
            );
            break;
          }
        }
      } catch (err) {
        showNotification(
          t('models_page.toast.model_update_failed', {
            model: row.id,
            error: getErrorMessage(err),
          }),
          'error'
        );
      } finally {
        setTogglingKeys((prev) => {
          const next = new Set(prev);
          next.delete(key);
          return next;
        });
      }
    },
    [fileStates, globalRulesByProvider, refetchConfig, showNotification, t, togglingKeys]
  );

  /* --------------------------- row computation --------------------------- */

  /**
   * Free-only fetch choice per provider card, mirroring the AI Providers
   * discovery panel. Defaults to ON for the OpenCode (free) group: its
   * catalog fetch keeps free models only.
   */
  const isFreeOnlyCard = useCallback(
    (card: CredentialCard): boolean =>
      freeOnlyByCard[card.id] ?? card.groupKey === 'opencodeFree',
    [freeOnlyByCard]
  );

  const displayCatalogs = useMemo(() => {
    const out: Record<string, CatalogEntryState> = {};
    for (const card of providerCards) {
      const state = catalogs[card.id];
      if (!state) continue;
      const filtered = applyFreeOnlyFilter(state.models, isFreeOnlyCard(card), {
        // OpenCode's catalog carries no pricing info, so "free only" falls
        // back to its `-free` id suffix instead of the pricing flag.
        inferSuffixFree: card.groupKey === 'opencodeFree',
      });
      out[card.id] =
        filtered.length === state.models.length
          ? state
          : { ...state, models: filtered };
    }
    return out;
  }, [catalogs, providerCards, isFreeOnlyCard]);

  const freeOnlyFlags = useMemo(() => {
    const out: Record<string, boolean> = {};
    for (const card of providerCards) out[card.id] = isFreeOnlyCard(card);
    return out;
  }, [providerCards, isFreeOnlyCard]);

  const setCardFreeOnly = useCallback((cardId: string, value: boolean) => {
    setFreeOnlyByCard((prev) => ({ ...prev, [cardId]: value }));
  }, []);

  const rowsByCard = useMemo(() => {
    const rows = new Map<string, ModelRow[]>();
    for (const card of providerCards) {
      rows.set(
        card.id,
        buildModelRows({
          card,
          discoveredNames: displayCatalogs[card.id]?.models.map((model) => model.name) ?? null,
        })
      );
    }
    const fileNamesByProvider = new Map<string, string[]>(
      oauthGroups.map((group) => [group.providerKey, group.files.map((file) => file.name)])
    );
    for (const card of oauthCards) {
      const providerKey = card.groupKey.replace(/^oauth:/, '');
      const channelState = channelCatalogs[providerKey];
      if (card.kind === 'oauth-group') {
        // Merged group card: one row list fed by the channel catalog (or the
        // union of per-account models when no static catalog exists) and the
        // union of every account's own rules plus the provider's global rules.
        const union = new Map<string, string>();
        const fileRulesUnion: string[] = [];
        for (const file of fileNamesByProvider.get(providerKey) ?? []) {
          const state = fileStates[file];
          (state?.catalogNames ?? []).forEach((name) => {
            if (name && !union.has(name.toLowerCase())) union.set(name.toLowerCase(), name);
          });
          for (const rule of state?.rules ?? []) {
            if (!fileRulesUnion.includes(rule)) fileRulesUnion.push(rule);
          }
        }
        rows.set(
          card.id,
          buildModelRows({
            card,
            catalogNames: channelState?.names?.length
              ? channelState.names
              : union.size
                ? Array.from(union.values())
                : null,
            fileRules: fileRulesUnion,
            globalRulesByProvider,
            oauthProviderKey: providerKey,
          })
        );
        continue;
      }
      const fileName = card.mutation.type === 'oauth-file' ? card.mutation.fileName : '';
      const fileState = fileName ? fileStates[fileName] : undefined;
      rows.set(
        card.id,
        buildModelRows({
          card,
          catalogNames: channelState?.names?.length
            ? channelState.names
            : fileState?.catalogNames ?? null,
          fileRules: fileState?.rules ?? [],
          globalRulesByProvider,
          oauthProviderKey: providerKey,
        })
      );
    }
    return rows;
  }, [channelCatalogs, displayCatalogs, fileStates, globalRulesByProvider, oauthCards, oauthGroups, providerCards]);

  return {
    connected,
    isPending,
    errorMessage,
    oauthError,
    groups,
    oauthGroups,
    catalogs,
    displayCatalogs,
    freeOnlyFlags,
    setCardFreeOnly,
    fileStates,
    channelCatalogs,
    globalRulesLoaded,
    globalRulesByProvider,
    fetchAllState,
    togglingKeys,
    rowsByCard,
    refetch,
    fetchCatalog,
    fetchAllCatalogs,
    ensureOAuthProviderData,
    refreshOAuthGroup,
    isOAuthLoaded: (group: OAuthGroup) => loadedOAuthRef.current.has(group.providerKey),
    toggleModel,
  };
}

export type { ModelInfo };
