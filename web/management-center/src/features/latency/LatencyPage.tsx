import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { latencyApi, type LatencyStats } from '@/services/api/latency';
import styles from './LatencyPage.module.scss';

const EMPTY: LatencyStats = {
  enabled: true,
  overall: { total: 0, failed: 0, errorRate: '0.0', p50Ms: 0, p95Ms: 0, p99Ms: 0, avgMs: 0, samples: 0 },
  byModel: [],
};

export function LatencyPage() {
  const { t } = useTranslation();
  const connected = useAuthStore((s) => s.connectionStatus) === 'connected';
  const [data, setData] = useState<LatencyStats>(EMPTY);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    setError('');
    try {
      const r = await latencyApi.get();
      setData(r ?? EMPTY);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [connected]);

  useEffect(() => {
    void load();
  }, [load]);

  const overall = data.overall ?? EMPTY.overall;
  const byModel = data.byModel ?? [];

  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <h1 className={styles.title}>{t('latency.title', 'Latency & Model Health')}</h1>
        <p className={styles.subtitle}>{t('latency.subtitle', 'p50/p95/p99 latency and per-model health (in-memory, resets on restart).')}</p>
      </div>

      <Card className={styles.card}>
        <h2 className={styles.cardTitle}>{t('latency.overall', 'Overall')}</h2>
        <div className={styles.statsGrid}>
          <div className={styles.stat}><span className={styles.label}>p50</span><span className={styles.value}>{overall.p50Ms}ms</span></div>
          <div className={styles.stat}><span className={styles.label}>p95</span><span className={styles.value}>{overall.p95Ms}ms</span></div>
          <div className={styles.stat}><span className={styles.label}>p99</span><span className={styles.value}>{overall.p99Ms}ms</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('latency.requests', 'Requests')}</span><span className={styles.value}>{overall.total}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('latency.errors', 'Errors')}</span><span className={styles.value}>{overall.failed}</span></div>
          <div className={styles.stat}><span className={styles.label}>{t('latency.error_rate', 'Error Rate')}</span><span className={styles.value}>{overall.errorRate}%</span></div>
        </div>
        <div className={styles.actions}>
          <Button onClick={load} disabled={loading}>{t('common.refresh', 'Refresh')}</Button>
        </div>
        {error ? <div className={styles.error}>{error}</div> : null}
      </Card>

      <Card className={styles.card}>
        <h2 className={styles.cardTitle}>{t('latency.by_model', 'By Model (Model Health)')}</h2>
        {byModel.length === 0 ? (
          <div className={styles.muted}>{t('latency.no_data', 'No latency samples yet.')}</div>
        ) : (
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Model</th>
                  <th>Req</th>
                  <th>Err</th>
                  <th>Err%</th>
                  <th>p50</th>
                  <th>p95</th>
                  <th>p99</th>
                  <th>Samples</th>
                </tr>
              </thead>
              <tbody>
                {byModel.map((m, i) => (
                  <tr key={i}>
                    <td>{m.provider}/{m.model}</td>
                    <td>{m.total}</td>
                    <td>{m.failed}</td>
                    <td>{m.errorRate}%</td>
                    <td>{m.p50Ms}ms</td>
                    <td>{m.p95Ms}ms</td>
                    <td>{m.p99Ms}ms</td>
                    <td>{m.samples}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
