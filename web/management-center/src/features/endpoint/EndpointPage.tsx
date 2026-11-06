import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores';
import { apiKeysApi } from '@/services/api';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import styles from './EndpointPage.module.scss';

export function EndpointPage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((s) => s.connectionStatus);
  const apiBase = useAuthStore((s) => s.apiBase);

  const [keys, setKeys] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (connectionStatus !== 'connected') return;
    setLoading(true);
    setError('');
    try {
      const list = await apiKeysApi.list();
      setKeys(list ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [connectionStatus]);

  useEffect(() => {
    void load();
  }, [load]);

  const baseUrl = apiBase || window.location.origin;
  const primaryKey = keys[0] ?? 'YOUR_API_KEY';

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* ignore */
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.hero}>
        <h1 className={styles.title}>{t('endpoint.title', 'Endpoint & Key')}</h1>
        <p className={styles.subtitle}>
          {t('endpoint.subtitle', 'Base URL, API keys, and quick-start examples for your XinuProxyApi gateway.')}
        </p>
      </div>

      <Card className={styles.card}>
        <h2 className={styles.cardTitle}>{t('endpoint.base_url', 'Base URL')}</h2>
        <div className={styles.row}>
          <code className={styles.code}>{baseUrl}</code>
          <Button size="sm" onClick={() => copy(baseUrl)}>{t('common.copy', 'Copy')}</Button>
        </div>
      </Card>

      <Card className={styles.card}>
        <h2 className={styles.cardTitle}>{t('endpoint.api_keys', 'API Keys')}</h2>
        {loading ? (
          <div className={styles.muted}>{t('common.loading', 'Loading…')}</div>
        ) : error ? (
          <div className={styles.error}>{error}</div>
        ) : keys.length === 0 ? (
          <div className={styles.muted}>{t('endpoint.no_keys', 'No API keys configured. Add one in Config.')}</div>
        ) : (
          <ul className={styles.keyList}>
            {keys.map((k, i) => (
              <li key={i} className={styles.keyRow}>
                <code className={styles.code}>{k}</code>
                <Button size="sm" variant="ghost" onClick={() => copy(k)}>{t('common.copy', 'Copy')}</Button>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card className={styles.card}>
        <h2 className={styles.cardTitle}>{t('endpoint.quick_start', 'Quick Start')}</h2>
        <pre className={styles.pre}>{`curl ${baseUrl}/v1/chat/completions \\
  -H "Authorization: Bearer ${primaryKey}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"oc/glm-5.3-flash-free","messages":[{"role":"user","content":"Hello"}]}'`}</pre>
        <Button size="sm" onClick={() => copy(`curl ${baseUrl}/v1/chat/completions -H "Authorization: Bearer ${primaryKey}" -H "Content-Type: application/json" -d '{"model":"oc/glm-5.3-flash-free","messages":[{"role":"user","content":"Hello"}]}'`)}>
          {t('common.copy', 'Copy')}
        </Button>
      </Card>
    </div>
  );
}
