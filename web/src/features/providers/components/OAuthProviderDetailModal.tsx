import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import {
  IconCopy,
  IconExternalLink,
  IconKey,
  IconRefreshCw,
  IconTrash2,
} from '@/components/ui/icons';
import { ProviderIcon } from './ProviderIcon';
import { oauthApi } from '@/services/api/oauth';
import { authFilesApi } from '@/services/api/authFiles';
import { vertexApi } from '@/services/api/vertex';
import { useNotificationStore } from '@/stores';
import { copyToClipboard } from '@/utils/clipboard';
import { getErrorMessage } from '@/utils/helpers';
import type { AuthFileItem } from '@/types/authFile';
import styles from './OAuthProviderDetailModal.module.scss';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  providerId: string;
  providerName: string;
  authFiles: AuthFileItem[];
  onRefreshAuthFiles: () => Promise<void>;
}

const CALLBACK_SUPPORTED = new Set<string>([
  'codex',
  'anthropic',
  'claude',
  'antigravity',
  'xai',
  'devin',
]);
const XAI_CALLBACK_URL = 'http://127.0.0.1:56121/callback';

const buildXaiCallbackUrl = (input: string, state?: string): string | null => {
  const trimmed = input.trim();
  if (!trimmed) return null;
  const callbackState = state?.trim();
  if (!callbackState) return null;
  const callbackUrl = new URL(XAI_CALLBACK_URL);
  callbackUrl.searchParams.set('code', trimmed);
  callbackUrl.searchParams.set('state', callbackState);
  return callbackUrl.toString();
};

export function OAuthProviderDetailModal({
  isOpen,
  onClose,
  providerId,
  providerName,
  authFiles,
  onRefreshAuthFiles,
}: Props) {
  const { showNotification, showConfirmation } = useNotificationStore();

  const [showLoginPanel, setShowLoginPanel] = useState(false);
  const [authUrl, setAuthUrl] = useState('');
  const [authState, setAuthState] = useState('');
  const [userCode, setUserCode] = useState('');
  const [status, setStatus] = useState<'idle' | 'waiting' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');
  const [callbackInput, setCallbackInput] = useState('');
  const [callbackSubmitting, setCallbackSubmitting] = useState(false);

  // Vertex Import states
  const [vertexFile, setVertexFile] = useState<File | null>(null);
  const [vertexLocation, setVertexLocation] = useState('us-central1');
  const [vertexLoading, setVertexLoading] = useState(false);

  // Per-account busy states
  const [busyAccounts, setBusyAccounts] = useState<Set<string>>(new Set());

  const pollTimerRef = useRef<number | null>(null);
  const pollStatusRef = useRef<(stateStr: string) => Promise<void>>(async () => {});

  const clearPolling = useCallback(() => {
    if (pollTimerRef.current !== null) {
      window.clearTimeout(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    return () => {
      clearPolling();
    };
  }, [clearPolling]);

  // Reset modal state when closed or provider changes
  useEffect(() => {
    if (!isOpen) {
      setShowLoginPanel(false);
      setAuthUrl('');
      setUserCode('');
      setStatus('idle');
      setErrorMessage('');
      setCallbackInput('');
      setVertexFile(null);
      clearPolling();
    }
  }, [isOpen, providerId, clearPolling]);

  const pollStatus = useCallback(
    async (stateStr: string) => {
      try {
        const res = await oauthApi.getAuthStatus(stateStr);
        if (res.status === 'ok') {
          setStatus('success');
          showNotification(`Successfully authenticated with ${providerName}!`, 'success');
          void onRefreshAuthFiles();
          setTimeout(() => {
            setShowLoginPanel(false);
            setStatus('idle');
          }, 3000);
          return;
        }
        if (res.status === 'error') {
          setStatus('error');
          setErrorMessage(res.error || 'Authorization failed');
          return;
        }

        // Still waiting, poll again in 2 seconds
        pollTimerRef.current = window.setTimeout(() => {
          void pollStatusRef.current(stateStr);
        }, 2000);
      } catch (err) {
        setStatus('error');
        setErrorMessage(getErrorMessage(err));
      }
    },
    [onRefreshAuthFiles, providerName, showNotification]
  );

  useEffect(() => {
    pollStatusRef.current = pollStatus;
  }, [pollStatus]);

  const handleStartAuth = async () => {
    clearPolling();
    setStatus('waiting');
    setErrorMessage('');
    setShowLoginPanel(true);

    // Map internal id to oauthApi provider string
    const targetProvider = providerId === 'claude' ? 'anthropic' : providerId;

    try {
      const res = await oauthApi.startAuth(targetProvider);
      if (res.url) {
        setAuthUrl(res.url);
        window.open(res.url, '_blank', 'width=650,height=750');
      }
      if (res.user_code) {
        setUserCode(res.user_code);
      }
      if (res.state) {
        setAuthState(res.state);
        // Start polling
        pollTimerRef.current = window.setTimeout(() => {
          void pollStatus(res.state!);
        }, 2000);
      }
    } catch (err) {
      setStatus('error');
      setErrorMessage(getErrorMessage(err));
    }
  };

  const handleSubmitCallback = async () => {
    if (!callbackInput.trim()) return;
    setCallbackSubmitting(true);
    setErrorMessage('');

    const targetProvider = providerId === 'claude' ? 'anthropic' : providerId;
    let finalUrl = callbackInput.trim();
    if (providerId === 'xai' && authState) {
      finalUrl = buildXaiCallbackUrl(callbackInput, authState) || finalUrl;
    }

    try {
      await oauthApi.submitCallback(targetProvider, finalUrl);
      showNotification(`Callback submitted successfully for ${providerName}!`, 'success');
      setCallbackInput('');
      void onRefreshAuthFiles();
      setShowLoginPanel(false);
      setStatus('idle');
    } catch (err) {
      setErrorMessage(getErrorMessage(err));
      showNotification(`Failed to submit callback: ${getErrorMessage(err)}`, 'error');
    } finally {
      setCallbackSubmitting(false);
    }
  };

  const handleVertexImport = async () => {
    if (!vertexFile) {
      showNotification('Please select a service account JSON file', 'error');
      return;
    }
    setVertexLoading(true);
    try {
      const res = await vertexApi.importCredential(vertexFile, vertexLocation);
      showNotification(
        `Vertex AI Service Account imported: ${res.auth_file || res['auth-file'] || res.email}`,
        'success'
      );
      setVertexFile(null);
      void onRefreshAuthFiles();
      setShowLoginPanel(false);
    } catch (err) {
      showNotification(`Vertex import failed: ${getErrorMessage(err)}`, 'error');
    } finally {
      setVertexLoading(false);
    }
  };

  const handleToggleAccount = async (account: AuthFileItem) => {
    const nextDisabled = !account.disabled;
    setBusyAccounts((prev) => new Set(prev).add(account.name));
    try {
      await authFilesApi.setStatus(account.name, nextDisabled);
      showNotification(
        `${account.email || account.name} ${nextDisabled ? 'disabled' : 'enabled'}`,
        'success'
      );
      void onRefreshAuthFiles();
    } catch (err) {
      showNotification(`Failed to update status: ${getErrorMessage(err)}`, 'error');
    } finally {
      setBusyAccounts((prev) => {
        const next = new Set(prev);
        next.delete(account.name);
        return next;
      });
    }
  };

  const handleRefreshAccountToken = async (account: AuthFileItem) => {
    setBusyAccounts((prev) => new Set(prev).add(account.name));
    try {
      await authFilesApi.requestManualRefresh(account.name, String(account.authIndex || ''));
      showNotification(`Token refreshed for ${account.email || account.name}`, 'success');
      void onRefreshAuthFiles();
    } catch (err) {
      showNotification(`Failed to refresh token: ${getErrorMessage(err)}`, 'error');
    } finally {
      setBusyAccounts((prev) => {
        const next = new Set(prev);
        next.delete(account.name);
        return next;
      });
    }
  };

  const handleDeleteAccount = (account: AuthFileItem) => {
    showConfirmation({
      title: 'Delete Account',
      message: `Are you sure you want to remove account "${account.email || account.name}"? This will delete the auth file permanently.`,
      confirmText: 'Delete',
      variant: 'danger',
      onConfirm: async () => {
        try {
          await authFilesApi.deleteFile(account.name);
          showNotification(`Removed account ${account.email || account.name}`, 'success');
          void onRefreshAuthFiles();
        } catch (err) {
          showNotification(`Failed to delete account: ${getErrorMessage(err)}`, 'error');
        }
      },
    });
  };

  const isVertex = providerId === 'vertex';
  const supportsCallback = CALLBACK_SUPPORTED.has(providerId === 'claude' ? 'anthropic' : providerId);

  return (
    <Modal open={isOpen} onClose={onClose} title="">
      <div className={styles.modalBody}>
        {/* Header */}
        <div className={styles.headerRow}>
          <div className={styles.headerLeft}>
            <ProviderIcon id={providerId} size="lg" />
            <div className={styles.headerTitles}>
              <h2 className={styles.title}>{providerName}</h2>
              <p className={styles.subtitle}>
                {authFiles.length} connected {authFiles.length === 1 ? 'account' : 'accounts'}
              </p>
            </div>
          </div>

          <Button
            size="sm"
            variant={showLoginPanel ? 'secondary' : 'primary'}
            onClick={() => {
              if (showLoginPanel) {
                setShowLoginPanel(false);
                clearPolling();
              } else if (isVertex) {
                setShowLoginPanel(true);
              } else {
                void handleStartAuth();
              }
            }}
          >
            {showLoginPanel ? 'Cancel Login' : isVertex ? 'Import Service Account' : 'Login with OAuth'}
          </Button>
        </div>

        {/* Login Panel */}
        {showLoginPanel && (
          <div className={styles.loginCard}>
            <div className={styles.loginCardTitle}>
              {isVertex ? 'Upload Google Vertex AI Service Account' : `Connect New ${providerName} Account`}
            </div>

            {isVertex ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                <Input
                  type="file"
                  accept=".json,application/json"
                  onChange={(e: ChangeEvent<HTMLInputElement>) => {
                    const f = e.target.files?.[0] || null;
                    setVertexFile(f);
                  }}
                />
                <Input
                  placeholder="Location (e.g. us-central1)"
                  value={vertexLocation}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => setVertexLocation(e.target.value)}
                />
                <Button size="sm" onClick={handleVertexImport} loading={vertexLoading}>
                  Upload & Import
                </Button>
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {authUrl && (
                  <div className={styles.authUrlRow}>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => window.open(authUrl, '_blank', 'width=650,height=750')}
                    >
                      <IconExternalLink size={14} /> Open Login Window
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={async () => {
                        await copyToClipboard(authUrl);
                        showNotification('Login URL copied to clipboard', 'success');
                      }}
                    >
                      <IconCopy size={14} /> Copy URL
                    </Button>
                  </div>
                )}

                {userCode && (
                  <div>
                    <span style={{ fontSize: 12, color: 'var(--text-secondary)' }}>Device Code:</span>
                    <div className={styles.userCodeBox}>
                      <span>{userCode}</span>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={async () => {
                          await copyToClipboard(userCode);
                          showNotification('Device code copied', 'success');
                        }}
                      >
                        <IconCopy size={14} />
                      </Button>
                    </div>
                  </div>
                )}

                {status === 'waiting' && (
                  <div className={styles.statusIndicator}>
                    <span className={styles.spinner} />
                    <span>Waiting for browser authorization...</span>
                  </div>
                )}

                {errorMessage && (
                  <div style={{ fontSize: 12, color: '#ef4444' }}>{errorMessage}</div>
                )}

                {supportsCallback && (
                  <div className={styles.callbackRow}>
                    <Input
                      placeholder="Paste redirect URL or authorization code here..."
                      value={callbackInput}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => setCallbackInput(e.target.value)}
                      className={styles.callbackInput}
                    />
                    <Button
                      size="sm"
                      onClick={handleSubmitCallback}
                      loading={callbackSubmitting}
                      disabled={!callbackInput.trim()}
                    >
                      Submit Code
                    </Button>
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        {/* Connected Accounts Section */}
        <div>
          <h3 className={styles.sectionTitle}>Connected Accounts</h3>
          {authFiles.length === 0 ? (
            <div className={styles.emptyAccounts}>
              <IconKey size={24} style={{ marginBottom: 6, opacity: 0.5 }} />
              <span>No accounts connected yet. Click “Login with OAuth” to connect your first account.</span>
            </div>
          ) : (
            <div className={styles.accountsList}>
              {authFiles.map((account) => {
                const isBusy = busyAccounts.has(account.name);
                return (
                  <div
                    key={account.name}
                    className={`${styles.accountRow} ${account.disabled ? styles.disabled : ''}`}
                  >
                    <div className={styles.accountMain}>
                      <span className={styles.accountEmail} title={account.name}>
                        {account.email || account.name}
                      </span>
                      <div className={styles.accountMeta}>
                        {account.email && <span>File: {account.name}</span>}
                        {account.priority !== undefined && <span>Priority: {account.priority}</span>}
                        {account.weight !== undefined && <span>Weight: {account.weight}</span>}
                        {account.disabled && <span style={{ color: '#ef4444' }}>[Disabled]</span>}
                        {account.unavailable && (
                          <span style={{ color: '#f59e0b' }}>[Unavailable / Cooldown]</span>
                        )}
                      </div>
                    </div>

                    <div className={styles.accountActions}>
                      <ToggleSwitch
                        checked={!account.disabled}
                        disabled={isBusy}
                        onChange={() => void handleToggleAccount(account)}
                        ariaLabel={`Toggle ${account.email || account.name}`}
                      />

                      <Button
                        size="sm"
                        variant="ghost"
                        title="Refresh Token"
                        disabled={isBusy}
                        onClick={() => void handleRefreshAccountToken(account)}
                      >
                        <IconRefreshCw size={14} className={isBusy ? styles.spinner : ''} />
                      </Button>

                      <Button
                        size="sm"
                        variant="ghost"
                        title="Delete Account"
                        disabled={isBusy}
                        onClick={() => handleDeleteAccount(account)}
                      >
                        <IconTrash2 size={14} style={{ color: '#ef4444' }} />
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
