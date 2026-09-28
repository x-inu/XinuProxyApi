/**
 * Shared application types.
 */

export type Theme = 'light' | 'dark';

export type NotificationType = 'info' | 'success' | 'warning' | 'error';

export interface Notification {
  id: string;
  message: string;
  type: NotificationType;
  duration?: number;
}
