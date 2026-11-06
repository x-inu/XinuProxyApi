import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores';
import { useNotificationStore } from '@/stores';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { cacheApi, type CacheStats } from '@/services/api/cache';
import styles from './CachePage.module.scss';

const EMPTY: CacheStats = {
  enabled: false,
  memoryEntries: 0,
  dbEntries: 0,
  hits: 0,
  misses: 0,
  hitRate: '0.0',
  tokensSaved: 0,
};

export function CachePage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((s) => s.connectionStatus);
  const connected = connectionStatus === 'connected';
  const showNotification = useNotificationStore((s) => s.showNotification);
  const [stats, setStats] = useState<CacheStats>(EMPTY);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    setError('');
    try {
      const data = await cacheApi.stats();
      setStats(data ?? EMPTY);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [connected]);

  useEffect(() => {
    void load();
  }, [load]);

  const clear = async () => {
    try {
      await cacheApi.clear();
      showNotification(t('common.success', 'Cleared'), 'success');
      await load();
    } catch (e) {
      showNotification(String(e), 'error');
    }
  };

  const hitRate = stats.hitRate ?? '0.0';

  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <h1 className={styles.title}>{t('cache.title', 'Semantic Cache')}</h1>
        <p className={styles.subtitle}>
          {t('cache.subtitle', 'Exact-match prompt/response cache (default disabled; enable in Config).')}
        </p>
      </div>

      <Card className={styles.card}>
        <div className={styles.statsGrid}>
          <div className={styles.stat}><span className={styles.label}>{t('cache.enabled', 'Enabled')}</span><span className={styles.value}>{stats.enabled ? 'Yes' : 'No'}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('cache.hits', 'Hits')}</span><span className={styles.value}>{stats.hits}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('cache.misses', 'Misses')}</span><span className={styles.value}>{stats.misses}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('cache.hit_rate', 'Hit Rate')}</span><span className={styles.value}>{hitRate}%</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('cache.tokens_saved', 'Tokens Saved')}</span><span className={styles.value}>{stats.tokensSaved}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('cache.entries', 'Entries')}</span><span className={styles.value}>{stats.memoryEntries}</span></div>
        </div>
        <div className={styles.actions}>
          <Button onClick={load} disabled={loading}>{t('common.refresh', 'Refresh')}</Button>
          <Button variant="danger" onClick={clear} disabled={!stats.memoryEntries}>{t('common.clear', 'Clear')}</Button>
        </div>
        {error ? <div className={styles.error}>{error}</div> : null}
      </Card>
    </div>
  );
}
