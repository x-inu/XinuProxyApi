import { useCallback, useEffect, useId, useImperativeHandle, useState, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { Sheet } from '@/components/ui/Sheet';
import { IconLoader2, IconPencil, IconPlus, IconTrash2 } from '@/components/ui/icons';
import type { ProviderRecentUsageMap } from '@/components/providers/utils';
import { useNotificationStore } from '@/stores';
import { PROVIDER_DESCRIPTORS } from '../descriptors';
import { isMultiProtocolSponsorBrand } from '../sponsorDefinitions';
import type { ProviderBrand, ProviderEntryFormInput, ProviderResource } from '../types';
import type { UseProviderWorkbenchResult } from '../useProviderWorkbench';
import { BaseProviderForm } from './forms/BaseProviderForm';
import { ResourceDetailView } from './ResourceDetailView';
import { SponsorProviderForm } from './forms/SponsorProviderForm';
import styles from './forms/sharedForm.module.scss';

type SheetMode = 'detail' | 'create' | 'edit';

export interface ProviderSheetState {
  open: boolean;
  brand: ProviderBrand;
  mode: SheetMode;
  resource: ProviderResource | null;
}

export interface ProviderSheetHandle {
  confirmDiscardIfDirty: () => Promise<boolean>;
}

interface ProviderSheetProps {
  state: ProviderSheetState;
  onClose: () => void;
  onSwitchToEdit: () => void;
  onSelectResource?: (resource: ProviderResource) => void;
  onCreateNew?: (brand: ProviderBrand) => void;
  onDelete?: (resource: ProviderResource) => void;
  brandResources?: ProviderResource[];
  workbench: UseProviderWorkbenchResult;
  onCreated: () => void;
  onUpdated: () => void;
  mutationDisabled?: boolean;
  usageByProvider?: ProviderRecentUsageMap;
  ref?: Ref<ProviderSheetHandle>;
}

export function ProviderSheet({
  state,
  onClose,
  onSwitchToEdit,
  onSelectResource,
  onCreateNew,
  onDelete,
  brandResources = [],
  workbench,
  onCreated,
  onUpdated,
  mutationDisabled = false,
  usageByProvider,
  ref,
}: ProviderSheetProps) {
  const { t } = useTranslation();
  const { showConfirmation } = useNotificationStore();
  const formId = useId();
  const [submitting, setSubmitting] = useState(false);
  const [isDirty, setIsDirty] = useState(false);

  // Reset dirty flag whenever the sheet is closed or the editing target
  // (brand / resource / mode) changes — the child form will re-mount and
  // re-report its own dirty state.
  useEffect(() => {
    setIsDirty(false);
  }, [state.brand, state.mode, state.resource?.id, state.open]);

  const handleDirtyChange = useCallback((dirty: boolean) => {
    setIsDirty(dirty);
  }, []);

  const descriptor = PROVIDER_DESCRIPTORS[state.brand];
  const isEditingForm = state.mode === 'create' || state.mode === 'edit';
  const formMutating = submitting || mutationDisabled;
  const submitDisabled = formMutating || (state.mode === 'edit' && !isDirty);

  const confirmDiscardIfDirty = useCallback((): Promise<boolean> => {
    if (!isEditingForm || !isDirty || submitting) {
      return Promise.resolve(true);
    }
    return new Promise<boolean>((resolve) => {
      showConfirmation({
        title: t('providersPage.unsavedChanges.title'),
        message: t('providersPage.unsavedChanges.message'),
        variant: 'danger',
        confirmText: t('providersPage.unsavedChanges.discard'),
        cancelText: t('providersPage.unsavedChanges.keepEditing'),
        onConfirm: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });
  }, [isDirty, isEditingForm, showConfirmation, submitting, t]);

  useImperativeHandle(ref, () => ({ confirmDiscardIfDirty }), [confirmDiscardIfDirty]);

  const handleCancelClick = useCallback(() => {
    void confirmDiscardIfDirty().then((ok) => {
      if (ok) onClose();
    });
  }, [confirmDiscardIfDirty, onClose]);

  const titleText =
    state.mode === 'create'
      ? `${t('providersPage.form.createEyebrow')} · ${t(
          `providersPage.providerNames.${state.brand}`
        )}`
      : state.mode === 'edit'
        ? `${t('providersPage.form.editEyebrow')} · ${t(
            `providersPage.providerNames.${state.brand}`
          )}`
        : `${t('providersPage.detail.title')} · ${t(`providersPage.providerNames.${state.brand}`)}`;

  const handleCreate = useCallback(
    async (input: ProviderEntryFormInput) => {
      if (mutationDisabled) return;
      setSubmitting(true);
      try {
        await workbench.createProvider(state.brand, input);
        onCreated();
      } finally {
        setSubmitting(false);
      }
    },
    [mutationDisabled, onCreated, state.brand, workbench]
  );

  const handleUpdate = useCallback(
    async (input: ProviderEntryFormInput) => {
      if (!state.resource || mutationDisabled || !isDirty) return;
      setSubmitting(true);
      try {
        await workbench.updateProvider(state.resource, input);
        onUpdated();
      } finally {
        setSubmitting(false);
      }
    },
    [isDirty, mutationDisabled, onUpdated, state.resource, workbench]
  );

  const renderResourceSwitcher = () => {
    if (isMultiProtocolSponsorBrand(state.brand)) return null;
    if (brandResources.length === 0 && state.mode === 'create') return null;

    const formatResourceEndpoint = (r: ProviderResource) => {
      if (r.baseUrl) return r.baseUrl;
      if (state.brand === 'claude') return 'https://api.anthropic.com (default)';
      return r.name || r.identifier;
    };

    const formatResourceKeyCount = (r: ProviderResource) => {
      const count = r.apiKeyEntryCount || (r.apiKey ? 1 : 0);
      if (count === 0) return '0 keys (empty)';
      return `${count} ${count === 1 ? 'key' : 'keys'}`;
    };

    return (
      <div
        className={styles.entryCard}
        style={{ marginBottom: 16 }}
        data-testid="provider-resource-switcher"
      >
        <div className={styles.entryCardHeader}>
          <span className={styles.sectionTitle}>
            Base URL Groups ({brandResources.length})
          </span>
          {state.mode !== 'create' && onCreateNew ? (
            <button
              type="button"
              className={styles.addBtn}
              disabled={formMutating}
              onClick={() => {
                void confirmDiscardIfDirty().then((ok) => {
                  if (ok) onCreateNew(state.brand);
                });
              }}
            >
              <IconPlus size={12} />
              <span>Add Base URL</span>
            </button>
          ) : null}
        </div>
        {brandResources.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {brandResources.map((r, idx) => {
              const isCurrent = state.mode !== 'create' && state.resource?.id === r.id;
              return (
                <div
                  key={r.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 8,
                    padding: '8px 10px',
                    borderRadius: 'var(--radius-md)',
                    border: isCurrent
                      ? '1px solid var(--primary-color)'
                      : '1px solid var(--border-color)',
                    background: isCurrent ? 'var(--primary-10)' : 'var(--bg-primary)',
                  }}
                >
                  <button
                    type="button"
                    className={styles.entryCardToggle}
                    onClick={() => {
                      if (isCurrent || !onSelectResource) return;
                      void confirmDiscardIfDirty().then((ok) => {
                        if (ok) onSelectResource(r);
                      });
                    }}
                  >
                    <span
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 6,
                        minWidth: 0,
                        flexWrap: 'wrap',
                      }}
                    >
                      <strong style={{ color: 'var(--text-primary)', fontSize: 12 }}>
                        #{idx + 1}
                      </strong>
                      {r.name ? (
                        <span className={styles.entryBadge}>{r.name}</span>
                      ) : null}
                      {r.prefix ? (
                        <span className={styles.entryBadge}>prefix: {r.prefix}</span>
                      ) : null}
                      <span
                        className={styles.entrySummaryKey}
                        style={{ maxWidth: 260 }}
                        title={formatResourceEndpoint(r)}
                      >
                        {formatResourceEndpoint(r)}
                      </span>
                    </span>
                    <span className={styles.entrySummary}>
                      <span className={styles.entryBadge}>{formatResourceKeyCount(r)}</span>
                      {r.disabled ? (
                        <span className={styles.entryBadge}>Disabled</span>
                      ) : null}
                    </span>
                  </button>
                  {onDelete ? (
                    <button
                      type="button"
                      className={styles.removeBtn}
                      disabled={formMutating}
                      title={t('providersPage.actions.delete')}
                      aria-label={t('providersPage.actions.delete')}
                      onClick={() => onDelete(r)}
                    >
                      <IconTrash2 size={13} />
                    </button>
                  ) : null}
                </div>
              );
            })}
          </div>
        ) : null}
      </div>
    );
  };

  const renderBody = () => {
    if (state.mode === 'detail') {
      if (!state.resource) {
        return null;
      }
      return (
        <>
          {renderResourceSwitcher()}
          <ResourceDetailView resource={state.resource} usageByProvider={usageByProvider} />
        </>
      );
    }
    const formKey = `${state.brand}:${state.resource?.id ?? 'new'}:${state.mode}`;
    if (isMultiProtocolSponsorBrand(state.brand)) {
      return (
        <SponsorProviderForm
          key={formKey}
          brand={state.brand}
          resource={state.resource}
          mode={state.mode}
          mutating={formMutating}
          formId={formId}
          onSubmit={state.mode === 'create' ? handleCreate : handleUpdate}
          onDirtyChange={handleDirtyChange}
        />
      );
    }
    return (
      <>
        {renderResourceSwitcher()}
        <BaseProviderForm
          key={formKey}
          brand={state.brand}
          resource={state.resource}
          mode={state.mode}
          mutating={formMutating}
          formId={formId}
          onSubmit={state.mode === 'create' ? handleCreate : handleUpdate}
          onDirtyChange={handleDirtyChange}
        />
      </>
    );
  };

  const footer =
    state.mode === 'detail' ? (
      state.resource ? (
        <>
          {onDelete ? (
            <button
              type="button"
              className={styles.removeBtn}
              style={{ marginRight: 'auto', height: 32, padding: '0 12px' }}
              onClick={() => onDelete(state.resource!)}
              disabled={formMutating}
            >
              <IconTrash2 size={14} />
              <span>{t('providersPage.actions.delete')}</span>
            </button>
          ) : null}
          <button
            type="button"
            className={`${styles.footerBtn} ${styles.footerBtnGhost}`}
            onClick={onClose}
          >
            {t('providersPage.actions.cancel')}
          </button>
          <button
            type="button"
            className={`${styles.footerBtn} ${styles.footerBtnPrimary}`}
            onClick={onSwitchToEdit}
            disabled={formMutating}
          >
            <IconPencil size={14} />
            {t('providersPage.actions.edit')}
          </button>
        </>
      ) : (
        <button
          type="button"
          className={`${styles.footerBtn} ${styles.footerBtnPrimary}`}
          onClick={onClose}
        >
          {t('providersPage.actions.cancel')}
        </button>
      )
    ) : (
      <>
        {state.mode === 'edit' && state.resource && onDelete ? (
          <button
            type="button"
            className={styles.removeBtn}
            style={{ marginRight: 'auto', height: 32, padding: '0 12px' }}
            onClick={() => onDelete(state.resource!)}
            disabled={formMutating}
          >
            <IconTrash2 size={14} />
            <span>{t('providersPage.actions.delete')}</span>
          </button>
        ) : null}
        <button
          type="button"
          className={`${styles.footerBtn} ${styles.footerBtnGhost}`}
          onClick={handleCancelClick}
          disabled={submitting}
        >
          {t('providersPage.actions.cancel')}
        </button>
        <button
          type="submit"
          form={formId}
          className={`${styles.footerBtn} ${styles.footerBtnPrimary}`}
          disabled={submitDisabled}
        >
          {submitting ? <IconLoader2 size={14} /> : null}
          {state.mode === 'create'
            ? t('providersPage.actions.create')
            : t('providersPage.actions.save')}
        </button>
      </>
    );

  return (
    <Sheet
      open={state.open}
      onClose={onClose}
      size={descriptor.sheetSize}
      eyebrow={
        state.mode === 'detail'
          ? t('providersPage.detail.title')
          : state.mode === 'create'
            ? t('providersPage.form.createEyebrow')
            : t('providersPage.form.editEyebrow')
      }
      title={titleText}
      description={t('providersPage.table.description', {
        route:
          state.brand === 'openaiCompatibility'
            ? '/ai-providers/openai'
            : state.brand === 'opencodeFree'
              ? '/ai-providers/opencode-free'
              : state.brand === 'fennoAI'
                ? '/ai-providers/fennoai'
                : state.brand === 'qiniuCloud'
                  ? '/ai-providers/qiniu'
                  : state.brand === 'kimi'
                    ? '/ai-providers/kimi'
                    : `/ai-providers/${state.brand}`,
      })}
      footer={footer}
      closeDisabled={submitting}
      confirmClose={confirmDiscardIfDirty}
    >
      {renderBody()}
    </Sheet>
  );
}
