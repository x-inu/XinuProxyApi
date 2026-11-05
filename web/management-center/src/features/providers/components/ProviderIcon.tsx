import { useState } from 'react';
import { useThemeStore } from '@/stores';
import { PROVIDER_LOGOS, NEEDS_DARK_SURFACE_ICONS, type ProviderBrandLogoKey } from '../brandLogos';
import { AUTH_FILE_ICONS, type AuthFileIconAsset } from '@/features/authFiles/constants';
import { IconPlug } from '@/components/ui/icons';

interface ProviderIconProps {
  id: string;
  apiType?: string;
  size?: 'sm' | 'md' | 'lg';
  iconSrc?: string;
  className?: string;
}

const SIZES = {
  sm: 24,
  md: 36,
  lg: 44,
};

export function ProviderIcon({
  id,
  size = 'md',
  iconSrc,
  className = '',
}: ProviderIconProps) {
  const resolvedTheme = useThemeStore((s) => s.resolvedTheme);
  const [loadError, setLoadError] = useState(false);

  const dim = SIZES[size];

  // 1. Explicit iconSrc provided
  if (iconSrc && !loadError) {
    return (
      <img
        src={iconSrc}
        alt=""
        width={dim}
        height={dim}
        className={className}
        onError={() => setLoadError(true)}
        style={{
          width: dim,
          height: dim,
          objectFit: 'contain',
          borderRadius: 6,
          flexShrink: 0,
        }}
      />
    );
  }

  // 2. Check PROVIDER_LOGOS
  const normKey = id.toLowerCase();
  const brandLogo = PROVIDER_LOGOS[id as ProviderBrandLogoKey] ?? PROVIDER_LOGOS[normKey as ProviderBrandLogoKey];
  if (brandLogo && !loadError) {
    const src =
      resolvedTheme === 'dark' && brandLogo.darkSrc ? brandLogo.darkSrc : brandLogo.src;
    const needsSurface = resolvedTheme === 'dark' && NEEDS_DARK_SURFACE_ICONS.has(id);

    return (
      <div
        style={{
          width: dim,
          height: dim,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          borderRadius: 6,
          backgroundColor: needsSurface ? '#ffffff' : 'transparent',
          padding: needsSurface ? 2 : 0,
        }}
        className={className}
      >
        <img
          src={src}
          alt=""
          width={dim}
          height={dim}
          onError={() => setLoadError(true)}
          style={{
            width: '100%',
            height: '100%',
            objectFit: 'contain',
          }}
        />
      </div>
    );
  }

  // 3. Check AUTH_FILE_ICONS
  const authIconAsset: AuthFileIconAsset | undefined =
    AUTH_FILE_ICONS[id] ?? AUTH_FILE_ICONS[normKey];
  if (authIconAsset && !loadError) {
    const src =
      typeof authIconAsset === 'string'
        ? authIconAsset
        : resolvedTheme === 'dark'
          ? authIconAsset.dark
          : authIconAsset.light;
    const needsSurface = resolvedTheme === 'dark' && NEEDS_DARK_SURFACE_ICONS.has(id);

    return (
      <div
        style={{
          width: dim,
          height: dim,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          borderRadius: 6,
          backgroundColor: needsSurface ? '#ffffff' : 'transparent',
          padding: needsSurface ? 2 : 0,
        }}
        className={className}
      >
        <img
          src={src}
          alt=""
          width={dim}
          height={dim}
          onError={() => setLoadError(true)}
          style={{
            width: '100%',
            height: '100%',
            objectFit: 'contain',
          }}
        />
      </div>
    );
  }

  // 4. Fallback plug / generic icon
  return (
    <div
      style={{
        width: dim,
        height: dim,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: 6,
        backgroundColor: 'var(--bg-tertiary)',
        color: 'var(--text-tertiary)',
        flexShrink: 0,
      }}
      className={className}
      aria-hidden="true"
    >
      <IconPlug size={dim * 0.55} />
    </div>
  );
}
