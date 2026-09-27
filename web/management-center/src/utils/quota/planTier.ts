import { normalizePlanType } from './parsers';


export type CodexPlanTier = 'elite' | 'premium' | 'plain';

export const PREMIUM_CODEX_PLAN_TYPES = new Set([
  'pro',
  'prolite',
  'pro-lite',
  'pro_lite',
  'self_serve_business_prolite',
]);



export const ELITE_CODEX_PLAN_TYPE = 'pro';


export function resolvePlanTier(planType: string | null | undefined): CodexPlanTier {
  const normalized = normalizePlanType(planType);
  if (!normalized) return 'plain';
  if (normalized === ELITE_CODEX_PLAN_TYPE) return 'elite';
  if (PREMIUM_CODEX_PLAN_TYPES.has(normalized)) return 'premium';
  return 'plain';
}
