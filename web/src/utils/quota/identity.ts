import type { AuthFileItem } from '@/types';
import { normalizeRecentRequestAuthIndex } from '@/utils/recentRequests';
import { stripJsonExtension } from '@/features/authFiles/identity';
import { isDevinFile } from './validators';

const QUOTA_IDENTITY_SEPARATOR = '\0';

/**
 * Cache identity is filename-based for every existing provider. Devin alone can
 * expose multiple credential identities from one physical file, distinguished
 * by auth_index.
 */
export function getQuotaCacheKey(file: AuthFileItem): string {
  if (!isDevinFile(file)) return file.name;
  const authIndex = normalizeRecentRequestAuthIndex(file.authIndex);
  return `${file.name}${QUOTA_IDENTITY_SEPARATOR}${authIndex ?? ''}`;
}

/**
 * Card/lane label. Prefers the account email so a fleet of many credentials
 * reads as a list of addresses (e.g. `vikogts0@gmail.com`) instead of raw
 * filenames like `antigravity-vikogts0@gmail.com.json`. Devin still appends its
 * disambiguator because several Devin identities can share one physical file.
 */
export function getQuotaDisplayName(file: AuthFileItem): string {
  const email = file.email?.trim();
  if (!isDevinFile(file)) {
    return email || stripJsonExtension(file.name);
  }
  const identity = email || normalizeRecentRequestAuthIndex(file.authIndex);
  return identity ? `${file.name} · ${identity}` : file.name;
}

/** Resolve a cache identity back to the physical filename used by file mutations. */
export function getQuotaCacheFileName(key: string): string {
  const separatorIndex = key.indexOf(QUOTA_IDENTITY_SEPARATOR);
  return separatorIndex === -1 ? key : key.slice(0, separatorIndex);
}
