import React, { useEffect, useMemo, useState, useCallback } from 'react';
import { Navigate, useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { SelectionCheckbox } from '@/components/ui/SelectionCheckbox';
import { IconEye, IconEyeOff } from '@/components/ui/icons';
import { useAuthStore, useNotificationStore } from '@/stores';
import { useThemeStore } from '@/stores/useThemeStore';
import { detectApiBaseFromLocation, normalizeApiBase } from '@/utils/connection';
import { INLINE_LOGO_JPEG } from '@/assets/logoInline';
import type { ApiError } from '@/types';
import styles from './LoginPage.module.scss';

function SunIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2" />
      <path d="M12 20v2" />
      <path d="m4.93 4.93 1.41 1.41" />
      <path d="m17.66 17.66 1.41 1.41" />
      <path d="M2 12h2" />
      <path d="M20 12h2" />
      <path d="m6.34 17.66-1.41 1.41" />
      <path d="m19.07 4.93-1.41 1.41" />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z" />
    </svg>
  );
}

/**
 * Converts API errors into user-friendly English messages.
 */
type RedirectState = { from?: { pathname?: string } };

function getLocalizedErrorMessage(error: unknown, t: (key: string) => string): string {
  const apiError = error as Partial<ApiError>;
  const status = typeof apiError.status === 'number' ? apiError.status : undefined;
  const code = typeof apiError.code === 'string' ? apiError.code : undefined;
  const message =
    error instanceof Error
      ? error.message
      : typeof apiError.message === 'string'
        ? apiError.message
        : typeof error === 'string'
          ? error
          : '';

  const withHttpStatus = (summary: string) => {
    if (!status) {
      return summary;
    }

    const genericAxiosMessage = `Request failed with status code ${status}`;
    const detail = message.trim();
    const backendDetail =
      detail && detail !== genericAxiosMessage
        ? ` (${t('login.error_backend_detail')}: ${detail})`
        : '';

    return `HTTP ${status}: ${summary}${backendDetail}`;
  };

  // Handle HTTP status codes.
  if (status === 401) {
    return withHttpStatus(t('login.error_unauthorized'));
  }
  if (status === 403) {
    return withHttpStatus(t('login.error_forbidden'));
  }
  if (status === 404) {
    return withHttpStatus(t('login.error_not_found'));
  }
  if (status && status >= 500) {
    return withHttpStatus(t('login.error_server'));
  }

  // Handle Axios error codes.
  if (code === 'ECONNABORTED' || message.toLowerCase().includes('timeout')) {
    return t('login.error_timeout');
  }
  if (code === 'ERR_NETWORK' || message.toLowerCase().includes('network error')) {
    return t('login.error_network');
  }
  if (code === 'ERR_CERT_AUTHORITY_INVALID' || message.toLowerCase().includes('certificate')) {
    return t('login.error_ssl');
  }

  // Handle CORS errors.
  if (message.toLowerCase().includes('cors') || message.toLowerCase().includes('cross-origin')) {
    return t('login.error_cors');
  }

  // Fall back to a generic error message.
  return withHttpStatus(t('login.error_invalid'));
}

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { showNotification } = useNotificationStore();
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const login = useAuthStore((state) => state.login);
  const restoreSession = useAuthStore((state) => state.restoreSession);
  const storedBase = useAuthStore((state) => state.apiBase);
  const storedKey = useAuthStore((state) => state.managementKey);
  const storedRememberPassword = useAuthStore((state) => state.rememberPassword);
  const theme = useThemeStore((state) => state.theme);
  const toggleTheme = useThemeStore((state) => state.toggleTheme);

  const [apiBase, setApiBase] = useState('');
  const [managementKey, setManagementKey] = useState('');
  const [showKey, setShowKey] = useState(false);
  const [rememberPassword, setRememberPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [autoLoading, setAutoLoading] = useState(true);
  const [autoLoginSuccess, setAutoLoginSuccess] = useState(false);
  const [error, setError] = useState('');

  const detectedBase = useMemo(() => detectApiBaseFromLocation(), []);

  useEffect(() => {
    const init = async () => {
      try {
        const autoLoggedIn = await restoreSession();
        if (autoLoggedIn) {
          setAutoLoginSuccess(true);
          // Delay navigation so the success animation remains visible.
          setTimeout(() => {
            const redirect = (location.state as RedirectState | null)?.from?.pathname || '/';
            navigate(redirect, { replace: true });
          }, 1500);
        } else {
          setApiBase(storedBase || detectedBase);
          setManagementKey(storedKey || '');
          setRememberPassword(storedRememberPassword || Boolean(storedKey));
        }
      } finally {
        // autoLoginSuccess keeps the splash visible after automatic login completes.
        setAutoLoading(false);
      }
    };

    init();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSubmit = useCallback(async () => {
    if (!managementKey.trim()) {
      setError(t('login.error_required'));
      return;
    }

    const baseToUse = apiBase ? normalizeApiBase(apiBase) : detectedBase;
    setLoading(true);
    setError('');
    try {
      await login({
        apiBase: baseToUse,
        managementKey: managementKey.trim(),
        rememberPassword,
      });
      showNotification(t('common.connected_status'), 'success');
      navigate('/', { replace: true });
    } catch (err: unknown) {
      const message = getLocalizedErrorMessage(err, t);
      setError(message);
      showNotification(`${t('notification.login_failed')}: ${message}`, 'error');
    } finally {
      setLoading(false);
    }
  }, [
    apiBase,
    detectedBase,
    login,
    managementKey,
    navigate,
    rememberPassword,
    showNotification,
    t,
  ]);

  const handleSubmitKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      if (event.key === 'Enter' && !loading) {
        event.preventDefault();
        handleSubmit();
      }
    },
    [loading, handleSubmit]
  );

  if (isAuthenticated && !autoLoading && !autoLoginSuccess) {
    const redirect = (location.state as RedirectState | null)?.from?.pathname || '/';
    return <Navigate to={redirect} replace />;
  }

  // Show the splash while automatic login runs or succeeds.
  const showSplash = autoLoading || autoLoginSuccess;

  return (
    <div className={styles.container}>
      {showSplash ? (
        /* Splash animation. */
        <div className={styles.splashScreen}>
          <div className={styles.splashContent}>
            <img src={INLINE_LOGO_JPEG} alt="XinuProxyApi" className={styles.splashLogo} />
            <h1 className={styles.splashTitle}>{t('splash.title')}</h1>
            <p className={styles.splashSubtitle}>{t('splash.subtitle')}</p>
            <div className={styles.splashLoader}>
              <div className={styles.splashLoaderBar} />
            </div>
          </div>
        </div>
      ) : (
        <>
          {/* Dotted wallpaper + oversized watermark glyph. */}
          <div className={styles.grid} aria-hidden="true" />
          <div className={styles.watermark} aria-hidden="true">
            源
          </div>

          {/* Fixed header: brand + theme toggle + status. */}
          <header className={styles.header}>
            <div className={styles.headerLogo}>
              <img src={INLINE_LOGO_JPEG} alt="" className={styles.headerGlyph} />
              <span>XinuProxyApi</span>
            </div>
            <div className={styles.headerRight}>
              <button
                type="button"
                className={styles.themeToggle}
                onClick={toggleTheme}
                aria-label={t('theme.switch')}
                title={t('theme.switch')}
              >
                {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
              </button>
              <div className={styles.status}>
                <i />
                SECURE SESSION
              </div>
            </div>
          </header>

          {/* Centered form dossier. */}
          <div className={styles.formWrapper}>
            <div className={styles.formKicker}>PROXY / ACCESS TERMINAL</div>
            <div className={styles.card}>
              <h1 className={styles.formTitle}>
                <span className={styles.titleLead}>Routed intelligence,</span>
                <span className={styles.titleAccent}>quietly governed.</span>
              </h1>
              <p className={styles.formDesc}>{t('login.subtitle')}</p>

              <div className={styles.formBody}>
                <Input
                  autoFocus
                  label={t('login.management_key_label')}
                  placeholder={t('login.management_key_placeholder')}
                  type={showKey ? 'text' : 'password'}
                  name="cpa-management-key"
                  autoComplete="current-password"
                  value={managementKey}
                  onChange={(e) => setManagementKey(e.target.value)}
                  onKeyDown={handleSubmitKeyDown}
                  rightElement={
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setShowKey((prev) => !prev)}
                      aria-label={
                        showKey
                          ? t('login.hide_key', { defaultValue: 'Hide key' })
                          : t('login.show_key', { defaultValue: 'Show key' })
                      }
                      title={
                        showKey
                          ? t('login.hide_key', { defaultValue: 'Hide key' })
                          : t('login.show_key', { defaultValue: 'Show key' })
                      }
                    >
                      {showKey ? <IconEyeOff size={16} /> : <IconEye size={16} />}
                    </button>
                  }
                />

                <div className={styles.toggleAdvanced}>
                  <SelectionCheckbox
                    checked={rememberPassword}
                    onChange={setRememberPassword}
                    ariaLabel={t('login.remember_password_label')}
                    label={t('login.remember_password_label')}
                    labelClassName={styles.toggleLabel}
                  />
                </div>

                <Button fullWidth onClick={handleSubmit} loading={loading}>
                  {loading ? t('login.submitting') : t('login.submit_button')}
                </Button>

                {error && <div className={styles.errorBox}>{error}</div>}
              </div>
            </div>
            <div className={styles.formMeta}>
              <span>PAPER {theme === 'dark' ? '#11100E' : '#F4F1EA'}</span>
              <span>INK {theme === 'dark' ? '#E6DED2' : '#1C1A17'}</span>
            </div>
          </div>

          <div className={styles.marginal} aria-hidden="true">
            XINUPROXYAPI / UNIFIED GATEWAY / EST. 2026
          </div>
        </>
      )}
    </div>
  );
}
