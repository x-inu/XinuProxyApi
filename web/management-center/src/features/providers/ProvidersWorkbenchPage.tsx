import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePageTransitionLayer } from '@/components/common/PageTransitionLayer';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { Skeleton } from '@/components/ui/Skeleton';
import { useAuthStore, useNotificationStore } from '@/stores';
import { useProviderRecentRequests } from '@/components/providers/hooks/useProviderRecentRequests';
import { ProviderHeaderCard } from './components/ProviderHeaderCard';
import { ProvidersOverviewGrid } from './components/ProvidersOverviewGrid';
import { OAuthProviderDetailModal } from './components/OAuthProviderDetailModal';
import { ProviderSheet, type ProviderSheetHandle } from './sheets/ProviderSheet';
import { useProviderWorkbench } from './useProviderWorkbench';
import { authFilesApi } from '@/services/api/authFiles';
import { getErrorMessage } from '@/utils/helpers';
import type { AuthFileItem } from '@/types/authFile';
import type { ProviderBrand, ProviderResource } from './types';
import styles from './ProvidersWorkbenchPage.module.scss';

type SheetMode = 'detail' | 'create' | 'edit';

interface SheetState {
  open: boolean;
  brand: ProviderBrand;
  mode: SheetMode;
  resource: ProviderResource | null;
}

const formatDateTime = (iso: string, locale?: string) => {
  try {
    const date = new Date(iso);
    if (Number.isNaN(date.getTime())) return iso;
    return new Intl.DateTimeFormat(locale, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(date);
  } catch {
    return iso;
  }
};

export function ProvidersWorkbenchPage() {
  const { t, i18n } = useTranslation();
  const connectionStatus = useAuthStore((s) => s.connectionStatus);
  const { showNotification } = useNotificationStore();

  const pageTransitionLayer = usePageTransitionLayer();
  const isCurrentLayer = pageTransitionLayer ? pageTransitionLayer.status === 'current' : true;

  const workbench = useProviderWorkbench();
  const [authFiles, setAuthFiles] = useState<AuthFileItem[]>([]);
  const [selectedOAuthProvider, setSelectedOAuthProvider] = useState<{ id: string; name: string } | null>(
    null
  );

  const [sheetState, setSheetState] = useState<SheetState>({
    open: false,
    brand: 'gemini',
    mode: 'detail',
    resource: null,
  });
  const sheetRef = useRef<ProviderSheetHandle>(null);

  const connected = connectionStatus === 'connected';
  const { usageByProvider, refreshRecentRequests } = useProviderRecentRequests({
    enabled: connected,
  });

  const refreshAuthFiles = useCallback(async () => {
    try {
      const res = await authFilesApi.list();
      setAuthFiles(res.files || []);
    } catch (err) {
      console.warn('Failed to load auth files for providers overview', err);
    }
  }, []);

  useEffect(() => {
    if (connected) {
      void refreshAuthFiles();
    }
  }, [connected, refreshAuthFiles]);

  const handleRefresh = useCallback(async () => {
    await Promise.allSettled([
      workbench.refetch(),
      refreshRecentRequests().catch(() => undefined),
      refreshAuthFiles().catch(() => undefined),
    ]);
  }, [refreshAuthFiles, refreshRecentRequests, workbench]);

  useHeaderRefresh(handleRefresh, isCurrentLayer);

  const disableMutations =
    connectionStatus !== 'connected' ||
    workbench.mutating ||
    workbench.isFetching ||
    workbench.isError;

  const groups = useMemo(() => workbench.snapshot?.groups ?? [], [workbench.snapshot]);

  const totalActive = useMemo(() => {
    const configActive = groups.reduce(
      (sum, g) => sum + g.resources.filter((r) => !r.disabled).length,
      0
    );
    const oauthActive = authFiles.filter((f) => !f.disabled && !f.unavailable).length;
    return configActive + oauthActive;
  }, [groups, authFiles]);

  const totalResources = useMemo(() => {
    const configTotal = groups.reduce((sum, g) => sum + g.resources.length, 0);
    const oauthTotal = authFiles.length;
    return configTotal + oauthTotal;
  }, [groups, authFiles]);

  const providerFamilies = useMemo(() => {
    const configFamilies = groups.filter((g) => g.resources.length > 0).length;
    const oauthSeen = new Set(
      authFiles.map((f) => (f.provider || f.type || '').toLowerCase()).filter(Boolean)
    );
    return configFamilies + oauthSeen.size;
  }, [groups, authFiles]);

  const updatedAtLabel = workbench.snapshot
    ? formatDateTime(workbench.snapshot.fetchedAt, i18n.language)
    : t('providersPage.modelCatalog.notLoaded');

  const errorBanner = workbench.errorMessage ? (
    <div className="error-box">{workbench.errorMessage}</div>
  ) : null;

  const openCreate = useCallback((brand: ProviderBrand = 'openaiCompatibility') => {
    setSheetState({ open: true, brand, mode: 'create', resource: null });
  }, []);

  const openEdit = useCallback((resource: ProviderResource) => {
    setSheetState({
      open: true,
      brand: resource.brand,
      mode: 'edit',
      resource,
    });
  }, []);

  const closeSheet = useCallback(() => {
    setSheetState((s) => ({ ...s, open: false }));
  }, []);

  const handleCreated = useCallback(() => {
    showNotification(t('providersPage.toast.created'), 'success');
    closeSheet();
  }, [closeSheet, showNotification, t]);

  const handleUpdated = useCallback(() => {
    showNotification(t('providersPage.toast.updated'), 'success');
    closeSheet();
  }, [closeSheet, showNotification, t]);

  const handleToggleConfig = useCallback(
    async (_brand: ProviderBrand, resource: ProviderResource, active: boolean) => {
      try {
        await workbench.toggleDisabled(resource, !active);
        showNotification(active ? 'Provider enabled' : 'Provider disabled', 'success');
      } catch (err) {
        showNotification(`Failed to toggle provider: ${getErrorMessage(err)}`, 'error');
      }
    },
    [workbench, showNotification]
  );

  const handleToggleOAuth = useCallback(
    async (providerId: string, active: boolean) => {
      const targetFiles = authFiles.filter((f) => {
        const norm = (f.provider || f.type || '').toLowerCase();
        if (providerId === 'claude') {
          return norm === 'claude' || norm === 'anthropic';
        }
        return norm === providerId.toLowerCase();
      });
      if (targetFiles.length === 0) return;

      try {
        await Promise.allSettled(
          targetFiles.map((f) => authFilesApi.setStatus(f.name, !active))
        );
        showNotification(
          active
            ? `${targetFiles.length} accounts enabled`
            : `${targetFiles.length} accounts disabled`,
          'success'
        );
        void refreshAuthFiles();
      } catch (err) {
        showNotification(`Failed to toggle accounts: ${getErrorMessage(err)}`, 'error');
      }
    },
    [authFiles, refreshAuthFiles, showNotification]
  );

  if (!workbench.snapshot && workbench.isPending) {
    return (
      <div className={styles.page}>
        <Skeleton height={120} />
        <Skeleton height={320} />
      </div>
    );
  }

  return (
    <div className={styles.page}>
      <ProviderHeaderCard
        totalActive={totalActive}
        totalResources={totalResources}
        providerFamilies={providerFamilies}
        updatedAtLabel={updatedAtLabel}
        isFetching={workbench.isFetching}
        isNewDisabled={disableMutations}
        showNewAction
        showSummary
        newLabel={t('providersPage.actions.new', { defaultValue: 'Add Custom Provider' })}
        onRefresh={() => void handleRefresh()}
        onNew={() => openCreate('openaiCompatibility')}
      />

      {errorBanner}

      {/* Grid of Provider Cards ("Kotak-kotak" 1:1 like XRoute) */}
      <ProvidersOverviewGrid
        groups={groups}
        authFiles={authFiles}
        onSelectConfigProvider={(brand, resource) => {
          if (resource) {
            openEdit(resource);
          } else {
            openCreate(brand);
          }
        }}
        onAddCustomProvider={() => openCreate('openaiCompatibility')}
        onToggleConfigProvider={handleToggleConfig}
        onSelectOAuthProvider={(providerId, providerName) => {
          setSelectedOAuthProvider({ id: providerId, name: providerName });
        }}
        onToggleOAuthProvider={handleToggleOAuth}
      />

      {/* OAuth Provider Detail & Login Modal */}
      {selectedOAuthProvider && (
        <OAuthProviderDetailModal
          isOpen={Boolean(selectedOAuthProvider)}
          onClose={() => setSelectedOAuthProvider(null)}
          providerId={selectedOAuthProvider.id}
          providerName={selectedOAuthProvider.name}
          authFiles={authFiles.filter((f) => {
            const norm = (f.provider || f.type || '').toLowerCase();
            if (selectedOAuthProvider.id === 'claude') {
              return norm === 'claude' || norm === 'anthropic';
            }
            return norm === selectedOAuthProvider.id.toLowerCase();
          })}
          onRefreshAuthFiles={refreshAuthFiles}
        />
      )}

      {/* Config Provider Sheet (Drawer) */}
      <ProviderSheet
        ref={sheetRef}
        state={sheetState}
        onClose={closeSheet}
        onSwitchToEdit={() => {
          setSheetState((s) => (s.resource ? { ...s, mode: 'edit' } : s));
        }}
        workbench={workbench}
        onCreated={handleCreated}
        onUpdated={handleUpdated}
        mutationDisabled={disableMutations}
        usageByProvider={usageByProvider}
      />
    </div>
  );
}
