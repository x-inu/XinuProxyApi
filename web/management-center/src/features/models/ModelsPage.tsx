import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Skeleton } from '@/components/ui/Skeleton';
import {
  IconAlertTriangle,
  IconChevronDown,
  IconDownload,
  IconLoader2,
  IconLock,
  IconRefreshCw,
  IconSearch,
} from '@/components/ui/icons';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { getTypeLabel } from '@/features/authFiles/constants';
import { PROVIDER_LOGOS } from '@/features/providers/brandLogos';
import { CredentialModelCard } from './CredentialModelCard';
import { useModelsCatalog, type OAuthGroup } from './useModelsCatalog';
import styles from './ModelsPage.module.scss';
import cardStyles from './CredentialModelCard.module.scss';

const EMPTY_CATALOG = {
  loading: false,
  error: null,
  models: [],
  fetchedAt: null,
};

export function ModelsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const catalog = useModelsCatalog();
  useHeaderRefresh(catalog.refetch);

  const [query, setQuery] = useState('');
  const [openProviderGroups, setOpenProviderGroups] = useState<Set<string>>(new Set());
  const [openOauthGroups, setOpenOauthGroups] = useState<Set<string>>(new Set());

  const providerCards = useMemo(
    () => catalog.groups.flatMap((group) => group.cards),
    [catalog.groups]
  );

  const fetchableCount = useMemo(
    () =>
      providerCards.filter(
        (card) => card.discovery && card.discovery.brand !== 'vertex'
      ).length,
    [providerCards]
  );

  const toggleProviderGroup = (groupKey: string) => {
    setOpenProviderGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupKey)) next.delete(groupKey);
      else next.add(groupKey);
      return next;
    });
  };

  const toggleOauthGroup = (group: OAuthGroup) => {
    const key = group.providerKey;
    const isOpen = openOauthGroups.has(key);
    setOpenOauthGroups((prev) => {
      const next = new Set(prev);
      if (isOpen) next.delete(key);
      else next.add(key);
      return next;
    });
    if (!isOpen) void catalog.ensureOAuthProviderData(group);
  };

  const providerGroupSection = (groupKey: string) => {
    const group = catalog.groups.find((candidate) => candidate.id === groupKey);
    if (!group) return null;
    const logo = PROVIDER_LOGOS[group.brand];
    const isOpen = openProviderGroups.has(group.id);
    const groupMatchesQuery = query.trim()
      ? group.cards.some((card) =>
          (catalog.rowsByCard.get(card.id) ?? []).some(
            (row) =>
              row.id.toLowerCase().includes(query.trim().toLowerCase()) ||
              (row.alias ?? '').toLowerCase().includes(query.trim().toLowerCase())
          )
        )
      : true;

    return (
      <div key={group.id} className={styles.group}>
        <button
          type="button"
          className={styles.groupHead}
          onClick={() => toggleProviderGroup(group.id)}
          aria-expanded={isOpen}
        >
          {logo ? (
            <>
              <img src={logo.src} alt="" aria-hidden="true" className={styles.logoLight} />
              {logo.darkSrc ? (
                <img
                  src={logo.darkSrc}
                  alt=""
                  aria-hidden="true"
                  className={styles.logoDark}
                />
              ) : null}
            </>
          ) : null}
          <span className={styles.groupTitle}>
            {t(`providersPage.providerNames.${group.brand}`)}
          </span>
          <span className={styles.groupCount}>{group.cards.length}</span>
          <IconChevronDown size={16} className={isOpen ? styles.chevronOpen : ''} />
        </button>
        {isOpen ? (
          groupMatchesQuery ? (
            <div className={styles.cardList}>
              {group.cards.map((card) => (
                <CredentialModelCard
                  key={card.id}
                  card={card}
                  rows={catalog.rowsByCard.get(card.id) ?? []}
                  catalog={catalog.catalogs[card.id] ?? EMPTY_CATALOG}
                  togglingKeys={catalog.togglingKeys}
                  query={query}
                  fetchingAll={catalog.fetchAllState.active}
                  onToggle={catalog.toggleModel}
                  onFetch={(target) => void catalog.fetchCatalog(target, true)}
                />
              ))}
            </div>
          ) : null
        ) : null}
      </div>
    );
  };

  const oauthGroupSection = (group: OAuthGroup) => {
    const isOpen = openOauthGroups.has(group.providerKey);
    const globalRules = catalog.globalRulesByProvider[group.providerKey] ?? [];
    const channelState = catalog.channelCatalogs[group.providerKey];

    return (
      <div key={`oauth-${group.providerKey}`} className={styles.group}>
        <div className={styles.groupHeadRow}>
          <button
            type="button"
            className={styles.groupHead}
            onClick={() => toggleOauthGroup(group)}
            aria-expanded={isOpen}
          >
            <span className={styles.groupTitle}>
              {getTypeLabel(t, group.providerKey)}
            </span>
            <span className={styles.groupCount}>{group.files.length}</span>
            <IconChevronDown size={16} className={isOpen ? styles.chevronOpen : ''} />
          </button>
          {isOpen ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => void catalog.refreshOAuthGroup(group)}
            >
              <IconRefreshCw size={14} />
              {t('models_page.oauth_group_refresh')}
            </Button>
          ) : null}
        </div>
        {isOpen ? (
          <div className={styles.cardList}>
            <div className={styles.globalRulesRow}>
              <span className={styles.globalRulesTitle}>
                {t('models_page.global_rules_title', { count: globalRules.length })}
              </span>
              {globalRules.map((rule) => (
                <code key={rule} className={cardStyles.wildcardChip}>
                  {rule}
                </code>
              ))}
              <Button
                variant="ghost"
                size="sm"
                onClick={() => navigate('/auth-files/oauth-excluded')}
              >
                {t('models_page.global_rules_open')}
              </Button>
            </div>
            {group.cards.map((card) => (
              <CredentialModelCard
                key={card.id}
                card={card}
                rows={catalog.rowsByCard.get(card.id) ?? []}
                catalog={EMPTY_CATALOG}
                togglingKeys={catalog.togglingKeys}
                query={query}
                fileState={catalog.fileStates[card.mutation.type === 'oauth-file' ? card.mutation.fileName : '']}
                channelState={channelState}
                globalRules={globalRules}
                globalRulesPending={!catalog.globalRulesLoaded}
                onToggle={catalog.toggleModel}
                onFetch={() => undefined}
                onRetryFile={() => void catalog.refreshOAuthGroup(group)}
                onOpenGlobalRules={() => navigate('/auth-files/oauth-excluded')}
              />
            ))}
          </div>
        ) : null}
      </div>
    );
  };

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.headerText}>
          <h1 className={styles.title}>{t('models_page.title')}</h1>
          <p className={styles.description}>{t('models_page.description')}</p>
        </div>
        <div className={styles.headerActions}>
          <div className={styles.searchWrap}>
            <IconSearch size={14} className={styles.searchIcon} />
            <Input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('models_page.search_placeholder')}
              aria-label={t('models_page.search_placeholder')}
              className={styles.searchInput}
            />
          </div>
          <Button
            onClick={() => void catalog.fetchAllCatalogs(providerCards)}
            disabled={catalog.fetchAllState.active || !catalog.connected}
          >
            {catalog.fetchAllState.active ? (
              <>
                <IconLoader2 size={16} className={cardStyles.spin} />
                {t('models_page.fetch_all_progress', {
                  done: catalog.fetchAllState.done,
                  total: catalog.fetchAllState.total,
                })}
              </>
            ) : (
              <>
                <IconDownload size={16} />
                {t('models_page.fetch_all_button', { count: fetchableCount })}
              </>
            )}
          </Button>
        </div>
      </header>

      {catalog.errorMessage ? (
        <div className={styles.errorBox} role="alert">
          <IconAlertTriangle size={16} />
          {catalog.errorMessage}
          <Button variant="ghost" onClick={() => void catalog.refetch()}>
            {t('models_page.retry')}
          </Button>
        </div>
      ) : null}

      {catalog.isPending ? (
        <div className={styles.skeletonBlock}>
          <Skeleton height={46} />
          <Skeleton height={220} />
          <Skeleton height={220} />
        </div>
      ) : (
        <>
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>{t('models_page.section_providers')}</h2>
            {catalog.groups.length === 0 ? (
              <div className={styles.emptySection}>{t('models_page.no_providers')}</div>
            ) : (
              catalog.groups.map((group) => providerGroupSection(group.id))
            )}
          </section>

          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>{t('models_page.section_oauth')}</h2>
            {catalog.oauthError ? (
              <div className={styles.errorBox} role="alert">
                <IconAlertTriangle size={16} />
                {catalog.oauthError}
              </div>
            ) : null}
            {catalog.oauthGroups.length === 0 ? (
              <div className={styles.emptySection}>{t('models_page.no_oauth_files')}</div>
            ) : (
              catalog.oauthGroups.map((group) => oauthGroupSection(group))
            )}
            <div className={styles.globalHint}>
              <IconLock size={12} />
              {t('models_page.catalog_note')}
            </div>
          </section>
        </>
      )}
    </div>
  );
}
