/**
 * Shared application constants.
 */

// Cache lifetime in milliseconds.
export const CACHE_EXPIRY_MS = 30 * 1000;

// Network and version metadata.
export const DEFAULT_API_PORT = 20128;
export const MANAGEMENT_API_PREFIX = '/v0/management';
export const REQUEST_TIMEOUT_MS = 30 * 1000;
export const XG_VERSION_HEADER_KEYS = ['x-xg-version'];
export const XG_BUILD_DATE_HEADER_KEYS = ['x-xg-build-date'];
export const XG_SUPPORT_PLUGIN_HEADER_KEYS = ['x-xg-support-plugin'];
export const VERSION_HEADER_KEYS = [...XG_VERSION_HEADER_KEYS, 'x-server-version'];
export const BUILD_DATE_HEADER_KEYS = [...XG_BUILD_DATE_HEADER_KEYS, 'x-server-build-date'];

// Log requests.
export const LOGS_TIMEOUT_MS = 60 * 1000;

// Authentication file upload limit.
export const MAX_AUTH_FILE_SIZE = 10 * 1024 * 1024;

// Browser storage keys.
export const STORAGE_KEY_AUTH = 'xinugate-auth';
export const STORAGE_KEY_SESSION = 'xinugate-session';
export const STORAGE_KEY_THEME = 'xinugate-theme';

// Notification lifetime.
export const NOTIFICATION_DURATION_MS = 3000;
