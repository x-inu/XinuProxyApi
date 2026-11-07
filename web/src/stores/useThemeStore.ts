

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { Theme } from '@/types';
import { STORAGE_KEY_THEME } from '@/utils/constants';

type ResolvedTheme = 'light' | 'dark';

interface ThemeState {
  theme: Theme;
  resolvedTheme: ResolvedTheme;
  setTheme: (theme: Theme) => void;
  toggleTheme: () => void;
  initializeTheme: () => () => void;
}

const applyTheme = (theme: Theme) => {
  if (theme === 'dark') {
    document.documentElement.setAttribute('data-theme', 'dark');
    return;
  }

  // Light = the ryoku paper palette (:root defaults, no data-theme attribute).
  document.documentElement.removeAttribute('data-theme');
};

// Duration and easing mirror the editorial scanline used across the sibling
// projects: a hard-edged repaint that advances in discrete rows.
const SCANLINE_DURATION = 260;
const SCANLINE_STEPS = 14;

const prefersReducedMotion = (): boolean =>
  typeof window !== 'undefined' &&
  !!window.matchMedia &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

let transitionRunning = false;

/**
 * Fallback scanline overlay for browsers without the View Transitions API.
 * A full-viewport panel of the incoming canvas color wipes top-to-bottom in
 * discrete steps, then the theme is committed.
 */
const runScanlineFallback = (commit: () => void): void => {
  const incomingIsDark = document.documentElement.getAttribute('data-theme') !== 'dark';
  const overlay = document.createElement('div');
  overlay.className = 'theme-scanline-fallback';
  overlay.style.background = incomingIsDark ? '#11100e' : '#f4f1ea';
  document.body.append(overlay);

  if (!overlay.animate) {
    overlay.remove();
    commit();
    return;
  }

  const animation = overlay.animate(
    [{ clipPath: 'inset(0 0 100% 0)' }, { clipPath: 'inset(0 0 0 0)' }],
    {
      duration: SCANLINE_DURATION,
      easing: `steps(${SCANLINE_STEPS}, end)`,
      fill: 'forwards',
    }
  );

  let switched = false;
  const finish = () => {
    if (switched) return;
    switched = true;
    commit();
    overlay.remove();
  };

  animation.onfinish = finish;
  animation.oncancel = finish;
};

/**
 * Animate a theme change. Uses the native View Transitions API when available
 * (styled as a scanline wipe in global.scss) and falls back to an overlay
 * animation otherwise. Instant when reduced motion is requested.
 */
const animateThemeChange = (commit: () => void): void => {
  if (transitionRunning) {
    commit();
    return;
  }

  if (prefersReducedMotion()) {
    commit();
    return;
  }

  transitionRunning = true;
  const done = () => {
    transitionRunning = false;
  };

  const startViewTransition = (
    document as Document & {
      startViewTransition?: (cb: () => void) => { finished: Promise<void> };
    }
  ).startViewTransition;

  if (typeof startViewTransition === 'function') {
    const transition = startViewTransition.call(document, commit);
    transition.finished.then(done, done);
    return;
  }

  runScanlineFallback(commit);
  window.setTimeout(done, SCANLINE_DURATION);
};

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'dark',
      resolvedTheme: 'dark',

      setTheme: (theme) => {
        if (get().theme === theme) return;
        animateThemeChange(() => {
          applyTheme(theme);
          set({ theme, resolvedTheme: theme });
        });
      },

      toggleTheme: () => {
        const next: Theme = get().theme === 'dark' ? 'light' : 'dark';
        get().setTheme(next);
      },

      initializeTheme: () => {
        // Normalize any legacy persisted value ('auto'/'white') down to the
        // two supported modes.
        const stored = get().theme;
        const theme: Theme = stored === 'dark' ? 'dark' : 'light';
        applyTheme(theme);
        set({ theme, resolvedTheme: theme });
        return () => {};
      },
    }),
    {
      name: STORAGE_KEY_THEME,
    }
  )
);
