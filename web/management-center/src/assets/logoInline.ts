// Inline brand mark for XinuProxyApi.
//
// Editorial paper-and-ink style matching the ryoku.dev-inspired theme: a crisp
// ink square with a hairline inset border and a bone 源 (source/origin) glyph,
// echoing the kanji marks used across the other Xinu projects. Delivered as an
// SVG data URI so it stays razor sharp at every size (favicon, sidebar, login).

const BRAND_MARK_SVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" role="img" aria-label="XinuProxyApi">
  <rect width="64" height="64" rx="3" fill="#1c1a17"/>
  <rect x="4.5" y="4.5" width="55" height="55" rx="1.5" fill="none" stroke="#c8862a" stroke-opacity="0.55" stroke-width="1.4"/>
  <text x="32" y="45" text-anchor="middle" font-family="'Iowan Old Style','Palatino','Georgia',serif" font-size="38" font-weight="600" fill="#f4f1ea">源</text>
</svg>`;

export const INLINE_LOGO_JPEG = `data:image/svg+xml,${encodeURIComponent(BRAND_MARK_SVG)}`;

export const INLINE_LOGO_SVG = INLINE_LOGO_JPEG;
