import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import { AUTH_FILE_ICONS } from '@/features/authFiles/constants';
import {
  PROVIDER_LOGOS,
  NEEDS_DARK_SURFACE_ICONS,
  type ProviderBrandLogoKey,
} from '@/features/providers/brandLogos';
import {
  IconAlertTriangle,
  IconLoader2,
  IconLock,
  IconRefreshCw,
} from '@/components/ui/icons';
import { DISABLE_ALL_RULE, isWildcardRule } from '@/components/excludedModels';
import type { CredentialCard, ModelRow } from './modelsCatalog';
import type { ChannelCatalogState, CatalogEntryState, FileRuleState } from './useModelsCatalog';
import styles from './CredentialModelCard.module.scss';

const PREVIEW_ROW_LIMIT = 15;

interface CredentialModelCardProps {
  card: CredentialCard;
  rows: ModelRow[];
  catalog: CatalogEntryState;
  togglingKeys: ReadonlySet<string>;
  query: string;
  fileState?: FileRuleState;
  channelState?: ChannelCatalogState;
  globalRulesPending?: boolean;
  fetchingAll?: boolean;
  freeOnly?: boolean;
  onFreeOnlyChange?: (value: boolean) => void;
  onToggle: (card: CredentialCard, row: ModelRow, nextEnabled: boolean) => void;
  onFetch: (card: CredentialCard, force: boolean) => void;
  onRetryFile?: (card: CredentialCard) => void;
  onOpenGlobalRules?: () => void;
}

const formatTime = (value: string | null): string => {
  if (!value) return '';
  try {
    return new Date(value).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return '';
  }
};

const originLabelKey: Record<ModelRow['origin'], string> = {
  configured: 'origin.configured',
  discovered: 'origin.discovered',
  catalog: 'origin.catalog',
  rule: 'origin.rule',
};

const matchQuery = (row: ModelRow, query: string): boolean => {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return (
    row.id.toLowerCase().includes(normalized) ||
    (row.alias ?? '').toLowerCase().includes(normalized)
  );
};

export function CredentialModelCard({
  card,
  rows,
  catalog,
  togglingKeys,
  query,
  fileState,
  channelState,
  globalRulesPending,
  fetchingAll,
  freeOnly,
  onFreeOnlyChange,
  onToggle,
  onFetch,
  onRetryFile,
  onOpenGlobalRules,
}: CredentialModelCardProps) {
  const { t } = useTranslation();
  const [expandedRows, setExpandedRows] = useState(false);

  const cardIcon = useMemo<{
    src: string;
    darkSrc?: string;
  } | null>(() => {
    if (card.kind === 'oauth-group') {
      const asset = card.mutation.type === 'oauth-group' ? AUTH_FILE_ICONS[card.mutation.providerKey] : undefined;
      if (!asset) return null;
      return typeof asset === 'string' ? { src: asset } : { src: asset.light, darkSrc: asset.dark };
    }
    return PROVIDER_LOGOS[card.brand as ProviderBrandLogoKey] ?? null;
  }, [card]);

  const cardIconOnDarkSurface =
    card.kind === 'oauth-group'
      ? card.mutation.type === 'oauth-group' && NEEDS_DARK_SURFACE_ICONS.has(card.mutation.providerKey)
      : NEEDS_DARK_SURFACE_ICONS.has(card.brand);

  const visibleRows = useMemo(() => rows.filter((row) => matchQuery(row, query)), [rows, query]);
  const onCount = rows.filter((row) => !row.excluded).length;
  const offCount = rows.length - onCount;
  const totalsMatchQuery = Boolean(query.trim());
  const showRows = totalsMatchQuery ? visibleRows : expandedRows ? rows : rows.slice(0, PREVIEW_ROW_LIMIT);
  const hiddenRowCount = rows.length - PREVIEW_ROW_LIMIT;
  const toggleKey = (row: ModelRow) => `${card.id}::${row.id}`;

  const wildcardRules = card.excludedRules.filter((rule) => isWildcardRule(rule));
  const fileOnlyRuleCount = rows.filter((row) => row.fileOnlyRule).length;

  const lockHint = (row: ModelRow): string | null => {
    if (card.lockDisabled) return t('models_page.lock.credential_disabled');
    if (row.globalRule) return t('models_page.lock.global', { rule: row.globalRule });
    if (row.wildcardRule) return t('models_page.lock.wildcard', { rule: row.wildcardRule });
    if (row.fileOnlyRule) return t('models_page.lock.per_account', { rule: row.fileOnlyRule });
    if (card.disableAll) return t('models_page.lock.disable_all', { rule: DISABLE_ALL_RULE });
    return null;
  };

  const catalogBusy = catalog.loading || fetchingAll;

  return (
    <div
      className={[styles.card, card.lockDisabled ? styles.cardLocked : ''].filter(Boolean).join(' ')}
    >
      <div className={styles.cardHead}>
        <div className={styles.cardTitleRow}>
          {cardIcon ? (
            cardIcon.darkSrc ? (
              <>
                <img
                  src={cardIcon.src}
                  alt=""
                  aria-hidden="true"
                  className={styles.brandIconLight}
                />
                <img
                  src={cardIcon.darkSrc}
                  alt=""
                  aria-hidden="true"
                  className={styles.brandIconDark}
                />
              </>
            ) : (
              <span
                className={
                  cardIconOnDarkSurface
                    ? `${styles.brandIconChip} ${styles.brandIconChipSurface}`
                    : styles.brandIconChip
                }
              >
                <img src={cardIcon.src} alt="" aria-hidden="true" />
              </span>
            )
          ) : null}
          <div className={styles.cardTitleBlock}>
            <span className={styles.cardTitle} title={card.mutation.type === 'oauth-file' ? card.title : undefined}>
              {card.title}
            </span>
            {card.subtitle ? <span className={styles.cardSubtitle}>{card.subtitle}</span> : null}
          </div>
        </div>
        <div className={styles.cardMeta}>
          <span className={styles.counterOn}>
            {t('models_page.count_on', { count: totalsMatchQuery ? visibleRows.filter((row) => !row.excluded).length : onCount })}
          </span>
          <span className={styles.counterOff}>
            {t('models_page.count_off', { count: totalsMatchQuery ? visibleRows.filter((row) => row.excluded).length : offCount })}
          </span>
        </div>
      </div>

      {card.kind === 'oauth-file' ? null : (
        <div className={styles.cardToolbar}>
          {card.discovery ? (
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onFetch(card, false)}
              disabled={catalogBusy}
              loading={catalog.loading}
            >
              {catalog.fetchedAt && !catalog.loading
                ? t('models_page.refetch')
                : t('models_page.fetch')}
            </Button>
          ) : (
            <span className={styles.noDiscoveryHint}>{t('models_page.no_discovery')}</span>
          )}
          {card.discovery?.brand === 'openaiCompatibility' && onFreeOnlyChange ? (
            <Button
              variant={freeOnly ? 'primary' : 'secondary'}
              size="sm"
              onClick={() => onFreeOnlyChange(!freeOnly)}
              disabled={catalog.loading}
              aria-pressed={freeOnly}
              aria-label={t('providersPage.discovery.fetchFreeOnly')}
            >
              {t('providersPage.discovery.fetchFreeOnly')}
            </Button>
          ) : null}
          {catalog.error && !catalog.loading ? (
            <span className={styles.catalogError} role="status">
              <IconAlertTriangle size={14} />
              {catalog.error}
            </span>
          ) : null}
          {catalog.fetchedAt && !catalog.loading ? (
            <span className={styles.catalogMeta}>
              {t('models_page.catalog_fetched', {
                count: catalog.models.length,
                time: formatTime(catalog.fetchedAt),
              })}
            </span>
          ) : null}
        </div>
      )}

      {card.kind === 'oauth-file' || card.kind === 'oauth-group' ? (
        <div className={styles.cardToolbar}>
          {fileState?.loading ? (
            <span className={styles.filePending}>
              <IconLoader2 size={14} className={styles.spin} />
              {t('models_page.file_rules_loading')}
            </span>
          ) : fileState?.error ? (
            <span className={styles.catalogError}>
              <IconAlertTriangle size={14} />
              {fileState.error}
              {onRetryFile ? (
                <Button variant="ghost" size="sm" onClick={() => onRetryFile(card)}>
                  {t('models_page.retry')}
                </Button>
              ) : null}
            </span>
          ) : null}
          {globalRulesPending ? (
            <span className={styles.filePending}>{t('models_page.global_rules_checking')}</span>
          ) : null}
          {channelState?.loading && !fileState?.loading ? (
            <span className={styles.filePending}>
              <IconLoader2 size={14} className={styles.spin} />
              {t('models_page.catalog_loading')}
            </span>
          ) : null}
          {fileOnlyRuleCount > 0 ? (
            <span className={styles.globalRuleNote}>
              <IconLock size={12} />
              {t('models_page.per_account_rules_note', { count: fileOnlyRuleCount })}
              {onOpenGlobalRules ? (
                <Button variant="ghost" size="sm" onClick={onOpenGlobalRules}>
                  {t('models_page.per_account_rules_open')}
                </Button>
              ) : null}
            </span>
          ) : null}
        </div>
      ) : (
        wildcardRules.length > 0 && (
          <div className={styles.wildcardRow}>
            <IconLock size={12} />
            <span className={styles.wildcardLabel}>{t('models_page.wildcard_rules_label')}</span>
            {wildcardRules.map((rule) => (
              <code key={rule} className={styles.wildcardChip}>
                {rule}
              </code>
            ))}
          </div>
        )
      )}

      <div className={styles.rowList} role="list">
        {showRows.length === 0 ? (
          <div className={styles.emptyRows}>
            {catalog.loading
              ? t('models_page.rows_loading')
              : totalsMatchQuery
                ? t('models_page.rows_no_match')
                : t('models_page.rows_empty')}
          </div>
        ) : (
          showRows.map((row) => {
            const busy = togglingKeys.has(toggleKey(row));
            const hint = lockHint(row);
            return (
              <div
                key={row.id}
                role="listitem"
                className={[
                  styles.row,
                  row.excluded ? styles.rowOff : '',
                  row.locked ? styles.rowLocked : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
              >
                <div className={styles.rowInfo}>
                  <code className={styles.rowId}>{row.id}</code>
                  {row.alias ? <span className={styles.rowAlias}>{row.alias}</span> : null}
                  {row.origin !== 'configured' ? (
                    <span className={styles.rowOrigin}>{t(`models_page.${originLabelKey[row.origin]}`)}</span>
                  ) : null}
                  {hint ? (
                    <span className={styles.rowHint} title={hint}>
                      <IconLock size={12} />
                      {hint}
                    </span>
                  ) : null}
                </div>
                <div className={styles.rowAction}>
                  {busy ? <IconLoader2 size={14} className={styles.spin} /> : null}
                  <ToggleSwitch
                    checked={!row.excluded}
                    disabled={row.locked || busy}
                    onChange={(value) => onToggle(card, row, value)}
                    ariaLabel={`${row.id} ${!row.excluded ? t('models_page.action_disable') : t('models_page.action_enable')}`}
                  />
                </div>
              </div>
            );
          })
        )}
      </div>

      {hiddenRowCount > 0 && !expandedRows && !totalsMatchQuery ? (
        <button
          type="button"
          className={styles.showMore}
          onClick={() => setExpandedRows(true)}
        >
          {t('models_page.show_more', { count: hiddenRowCount })}
        </button>
      ) : null}

      {card.kind === 'oauth-file' || card.kind === 'oauth-group' ? null : (
        <div className={styles.cardFooter}>
          {card.discovery ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onFetch(card, true)}
              disabled={catalogBusy}
            >
              <IconRefreshCw size={14} />
              {t('models_page.reload_catalog')}
            </Button>
          ) : null}
        </div>
      )}
    </div>
  );
}
