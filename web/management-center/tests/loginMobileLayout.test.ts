import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const pageSource = readFileSync(
  new URL('../src/pages/LoginPage.tsx', import.meta.url),
  'utf8'
);

const scssSource = readFileSync(
  new URL('../src/pages/LoginPage.module.scss', import.meta.url),
  'utf8'
);

const indexHtmlSource = readFileSync(
  new URL('../index.html', import.meta.url),
  'utf8'
);

describe('login and splash screen mobile viewport layout', () => {
  test('index.html viewport supports safe areas with viewport-fit=cover', () => {
    expect(indexHtmlSource).toContain('viewport-fit=cover');
  });

  test('splash screen renders independently from container padding', () => {
    // When showSplash is active, it must not be wrapped inside .container
    expect(pageSource).toContain('if (showSplash) {');
    expect(pageSource).toContain('<div className={styles.splashScreen} role="status" aria-live="polite">');
  });

  test('splash screen styles enforce fixed viewport and prevent scrolling', () => {
    expect(scssSource).toContain('.splashScreen {');
    expect(scssSource).toContain('position: fixed;');
    expect(scssSource).toContain('height: 100dvh;');
    expect(scssSource).toContain('touch-action: none;');
    expect(scssSource).toContain('overscroll-behavior: none;');
    expect(scssSource).toContain('overflow: hidden;');
  });

  test('container styles use 100dvh, safe area insets, and safe flex centering', () => {
    expect(scssSource).toContain('min-height: 100dvh;');
    expect(scssSource).toContain('overscroll-behavior-y: contain;');
    expect(scssSource).toContain('safe-area-inset-top');
    expect(scssSource).toContain('safe-area-inset-bottom');
    expect(scssSource).toContain('margin: auto 0;');
  });

  test('mobile card and title scaling are constrained to prevent offscreen overflow', () => {
    expect(scssSource).toContain('clamp(22px, 6.2vw, 26px)');
    expect(scssSource).toContain('font-size: 16px;'); // iOS Safari auto-zoom prevention
  });
});
