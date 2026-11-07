import { useMemo } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { useCountUp, useRevealGroup, useRevealOnScroll } from '@/hooks/motion';
import { formatCompactNumber, formatDateTimeValue } from '@/utils/format';
import { providerLabel } from '@/features/dashboard/utils';
import { Sparkline } from '@/features/dashboard/components/Sparkline';
import { useUsageOverview } from './useUsageOverview';
import styles from './usage.module.scss';

const DASH = '—';

const formatTokens = (value: number): string =>
  value < 100_000 ? value.toLocaleString() : formatCompactNumber(value);

const formatCost = (value: number): string => {
  if (!Number.isFinite(value) || value <= 0) return '$0.00';
  if (value < 0.01) return `<$0.01`;
  if (value < 1000) return `$${value.toFixed(2)}`;
  return `$${formatCompactNumber(value)}`;
};

export function UsagePage() {
  const { t, i18n } = useTranslation();
  const { connected, usage, loading, error, refresh } = useUsageOverview();

  useHeaderRefresh(refresh, connected);

  const heroRef = useRevealGroup<HTMLElement>();
  const statsRef = useRevealGroup<HTMLElement>(0.12);
  const trendRef = useRevealOnScroll<HTMLElement>();
  const modelsRef = useRevealOnScroll<HTMLElement>();

  const animatedTokens = useCountUp(usage.totals.totalTokens, connected);

  const unknownProviderLabel = t('dashboard.provider_unknown');

  const trendPoints = useMemo(
    () => usage.timeline.map((point) => point.totalTokens),
    [usage.timeline]
  );

  const costPoints = useMemo(
    () => usage.timeline.map((point) => point.costUsd),
    [usage.timeline]
  );

  const activeWindow = useMemo(() => {
    if (usage.timeline.length === 0) return DASH;
    const first = usage.timeline[0].hour;
    const last = usage.timeline[usage.timeline.length - 1].hour;
    const firstLabel = formatDateTimeValue(first, i18n.language) || DASH;
    const lastLabel = formatDateTimeValue(last, i18n.language) || DASH;
    if (firstLabel === lastLabel) return firstLabel;
    return `${firstLabel} → ${lastLabel}`;
  }, [usage.timeline, i18n.language]);

  const successCount = usage.totals.requests - usage.totals.failed;
  const successRate =
    usage.totals.requests > 0 ? (successCount / usage.totals.requests) * 100 : null;

  const statTiles = [
    {
      key: 'requests',
      label: t('usage.stat_requests'),
      value: connected ? usage.totals.requests.toLocaleString() : DASH,
      hint: t('usage.stat_requests_hint', {
        success: successCount.toLocaleString(),
        failed: usage.totals.failed.toLocaleString(),
      }),
    },
    {
      key: 'input',
      label: t('usage.stat_input'),
      value: connected ? formatTokens(usage.totals.inputTokens) : DASH,
      hint: t('usage.stat_input_hint', {
        cached: formatTokens(usage.totals.cachedTokens),
      }),
    },
    {
      key: 'output',
      label: t('usage.stat_output'),
      value: connected ? formatTokens(usage.totals.outputTokens) : DASH,
      hint: t('usage.stat_output_hint'),
    },
    {
      key: 'success',
      label: t('usage.stat_success_rate'),
      value: successRate === null ? DASH : `${successRate.toFixed(1).replace(/\.0$/, '')}%`,
      hint: t('usage.stat_success_rate_hint'),
    },
  ];

  const isEmpty = connected && usage.enabled && usage.byModel.length === 0;

  return (
    <div className={styles.page}>
      <div className={styles.ambient} aria-hidden="true">
        <span className={styles.washTop} />
        <span className={styles.gridWash} />
      </div>

      {/* ---------- Hero ---------- */}
      <section className={styles.hero} ref={heroRef}>
        <div className={styles.heroCopy}>
          <h1 className={styles.heroTitle} data-reveal>
            {t('usage.hero_title')}
          </h1>
          <p className={styles.heroMeta} data-reveal>
            {connected ? t('usage.hero_meta', { window: activeWindow }) : t('common.disconnected')}
          </p>
          <div className={styles.heroActions} data-reveal>
            <Link to="/" className={styles.ghostAction}>
              {t('usage.back_to_dashboard')}{' '}
              <span className={styles.linkArrow} aria-hidden="true">
                →
              </span>
            </Link>
          </div>
        </div>

        <div className={styles.heroPanel} data-reveal="scale">
          <div className={styles.heroPanelTop}>
            <span className={styles.heroPanelLabel}>{t('usage.hero_cost_label')}</span>
            <span className={styles.estBadge}>{t('usage.estimate_badge')}</span>
          </div>
          <strong className={styles.heroFigure}>
            {connected ? formatCost(usage.totals.costUsd) : DASH}
          </strong>
          <span className={styles.heroPanelMeta}>
            {t('usage.hero_tokens_meta', {
              tokens: connected ? formatTokens(animatedTokens) : DASH,
            })}
          </span>
        </div>
      </section>

      {/* ---------- KPI ---------- */}
      <section className={styles.statsRow} ref={statsRef} aria-label={t('usage.stats_aria')}>
        {statTiles.map((tile) => (
          <article key={tile.key} className={styles.statTile} data-reveal>
            <span className={styles.statLabel}>{tile.label}</span>
            <strong className={styles.statValue}>{tile.value}</strong>
            <span className={styles.statHint}>{tile.hint}</span>
          </article>
        ))}
      </section>

      {/* ---------- Disabled / empty notices ---------- */}
      {connected && !usage.enabled && !loading && (
        <section className={styles.section}>
          <div className={styles.panel}>
            <p className={styles.emptyNote}>{t('usage.disabled_note')}</p>
          </div>
        </section>
      )}

      {error && (
        <section className={styles.section}>
          <div className={styles.panel}>
            <p className={styles.emptyNote}>{t('usage.error_note')}</p>
          </div>
        </section>
      )}

      {/* ---------- Trend ---------- */}
      {connected && usage.enabled && (
        <section className={styles.section} ref={trendRef}>
          <header className={styles.sectionHead}>
            <span className={styles.eyebrow}>{t('usage.trend_eyebrow')}</span>
            <h2 className={styles.sectionTitle}>{t('usage.trend_title')}</h2>
            <p className={styles.sectionDescription}>{t('usage.trend_description')}</p>
          </header>
          <div className={styles.panel}>
            {isEmpty ? (
              <p className={styles.emptyNote}>{t('usage.trend_empty')}</p>
            ) : (
              <div className={styles.trendGrid}>
                <div className={styles.trendCell}>
                  <span className={styles.trendLabel}>{t('usage.trend_tokens')}</span>
                  <Sparkline
                    points={trendPoints}
                    ariaLabel={t('usage.trend_tokens')}
                    className={styles.trendSpark}
                  />
                </div>
                <div className={styles.trendCell}>
                  <span className={styles.trendLabel}>{t('usage.trend_cost')}</span>
                  <Sparkline
                    points={costPoints}
                    color="var(--amber-color)"
                    ariaLabel={t('usage.trend_cost')}
                    className={styles.trendSpark}
                  />
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {/* ---------- By model ---------- */}
      {connected && usage.enabled && (
        <section className={styles.section} ref={modelsRef}>
          <header className={styles.sectionHead}>
            <span className={styles.eyebrow}>{t('usage.models_eyebrow')}</span>
            <h2 className={styles.sectionTitle}>{t('usage.models_title')}</h2>
            <p className={styles.sectionDescription}>{t('usage.models_description')}</p>
          </header>
          <div className={styles.panel}>
            {usage.byModel.length === 0 ? (
              <p className={styles.emptyNote}>{t('usage.models_empty')}</p>
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.thLeft}>{t('usage.col_model')}</th>
                      <th>{t('usage.col_requests')}</th>
                      <th>{t('usage.col_input')}</th>
                      <th>{t('usage.col_output')}</th>
                      <th>{t('usage.col_total')}</th>
                      <th>{t('usage.col_cost')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {usage.byModel.map((row) => (
                      <tr key={`${row.provider}\u0000${row.model}`}>
                        <td className={styles.thLeft}>
                          <span className={styles.modelName}>{row.model}</span>
                          <span className={styles.modelProvider}>
                            {providerLabel(row.provider, unknownProviderLabel)}
                          </span>
                        </td>
                        <td className={styles.num}>{row.requests.toLocaleString()}</td>
                        <td className={styles.num}>{formatTokens(row.inputTokens)}</td>
                        <td className={styles.num}>{formatTokens(row.outputTokens)}</td>
                        <td className={styles.num}>{formatTokens(row.totalTokens)}</td>
                        <td className={styles.num}>{formatCost(row.costUsd)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <p className={styles.disclaimer}>{t('usage.cost_disclaimer')}</p>
          </div>
        </section>
      )}
    </div>
  );
}
