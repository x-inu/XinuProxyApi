import React from 'react';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import { ProviderIcon } from './ProviderIcon';
import styles from './ProviderCard.module.scss';

export interface ProviderStats {
  connected: number;
  errorCount: number;
  disabledCount: number;
  total: number;
  allDisabled: boolean;
}

interface ProviderCardProps {
  id: string;
  name: string;
  apiType?: string;
  stats: ProviderStats;
  noAuth?: boolean;
  iconSrc?: string;
  onClick: () => void;
  onToggleAll?: (active: boolean) => void;
}

export function ProviderCard({
  id,
  name,
  apiType,
  stats,
  noAuth = false,
  iconSrc,
  onClick,
  onToggleAll,
}: ProviderCardProps) {
  const isAllDisabled = stats.allDisabled;
  const cardStatusClass = isAllDisabled
    ? styles.disabled
    : stats.errorCount > 0
      ? styles.error
      : stats.connected > 0 || noAuth
        ? styles.connected
        : '';

  const handleToggleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
  };

  const handleToggleChange = (checked: boolean) => {
    onToggleAll?.(checked);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
      className={`${styles.card} ${cardStatusClass}`}
    >
      <div className={styles.left}>
        <ProviderIcon id={id} apiType={apiType} iconSrc={iconSrc} size="md" />

        <div className={styles.info}>
          <div className={styles.nameRow}>
            <span className={styles.name} title={name}>
              {name}
            </span>
            {apiType ? <span className={styles.apiTypeBadge}>{apiType}</span> : null}
          </div>

          <div className={styles.statusRow}>
            {isAllDisabled ? (
              <span className={`${styles.dotBadge} ${styles.disabled}`}>
                <span className={styles.dot} />
                Disabled
              </span>
            ) : stats.errorCount > 0 ? (
              <>
                <span className={`${styles.dotBadge} ${styles.error}`}>
                  <span className={styles.dot} />
                  {stats.errorCount} Error
                </span>
                {stats.connected > 0 ? (
                  <span className={`${styles.dotBadge} ${styles.success}`}>
                    <span className={styles.dot} />
                    {stats.connected} Connected
                  </span>
                ) : null}
              </>
            ) : noAuth ? (
              <span className={`${styles.dotBadge} ${styles.success}`}>
                <span className={styles.dot} />
                Ready
              </span>
            ) : stats.connected > 0 ? (
              <span className={`${styles.dotBadge} ${styles.success}`}>
                <span className={styles.dot} />
                {stats.connected} Connected
              </span>
            ) : (
              <span className={styles.noConn}>No connections</span>
            )}
          </div>
        </div>
      </div>

      {stats.total > 0 && onToggleAll ? (
        <div className={styles.right} onClick={handleToggleClick}>
          <ToggleSwitch
            checked={!isAllDisabled}
            onChange={handleToggleChange}
            ariaLabel={`Toggle all ${name} accounts`}
          />
        </div>
      ) : null}
    </div>
  );
}
