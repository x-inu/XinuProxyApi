


import { useCallback, useEffect, useRef, useState } from 'react';
import { prefersReducedMotion } from '@/hooks/motion';
import type { VisualConfigValues } from '@/types/visualConfig';
import { FIELD_HIGHLIGHT_CLASS } from '../components/fields/FieldPrimitives';
import type { ConfigTabId } from '../constants';
import {
  configFieldDomId,
  type ConfigFieldSearchEntry,
  type VisualSectionId,
} from '../searchIndex';

export type UseFieldJumpArgs = {
  values: VisualConfigValues;

  setActiveSection: (id: ConfigTabId) => void;
};

export function useFieldJump({ values, setActiveSection }: UseFieldJumpArgs) {
  // A fresh object per jump; the effect handles it once (guarded by handledJumpRef) so it
  // never needs to clear state from inside the effect.
  const [jumpRequest, setJumpRequest] = useState<{
    fieldId: string;
    sectionId: VisualSectionId;
  } | null>(null);
  const handledJumpRef = useRef<{ fieldId: string; sectionId: VisualSectionId } | null>(null);
  const highlightTimerRef = useRef<number | null>(null);
  const highlightedElRef = useRef<HTMLElement | null>(null);

  const jumpToField = useCallback(
    (entry: ConfigFieldSearchEntry) => {

      setActiveSection(entry.sectionId);
      setJumpRequest({ fieldId: entry.fieldId, sectionId: entry.sectionId });
    },
    [setActiveSection]
  );

  // Imperatively scroll to and pulse-highlight the jumped-to field once the target
  // section tab has mounted.
  useEffect(() => {
    if (!jumpRequest || handledJumpRef.current === jumpRequest) return;
    handledJumpRef.current = jumpRequest; // handle each request once, even if deps re-fire
    const { fieldId } = jumpRequest;

    const targetFieldId =
      (fieldId === 'tlsCert' || fieldId === 'tlsKey') && !values.tlsEnable ? 'tlsEnable' : fieldId;

    const attempt = (retriesLeft: number) => {
      const el = document.getElementById(configFieldDomId(targetFieldId));
      if (!el) {

        if (retriesLeft > 0) requestAnimationFrame(() => attempt(retriesLeft - 1));
        return;
      }

      // Expand the collapsed <details> group this field belongs to: an ancestor when the
      // anchor sits inside the group (TLS / remote / advanced fields), or a descendant when
      // the anchor wraps the whole group (payload rule groups).
      const details = el.closest('details') ?? el.querySelector('details');
      if (details && !details.open) details.open = true;

      // Clear any in-flight highlight before starting a new one.
      if (highlightTimerRef.current !== null) {
        clearTimeout(highlightTimerRef.current);
        highlightedElRef.current?.classList.remove(FIELD_HIGHLIGHT_CLASS);
      }

      el.scrollIntoView({
        behavior: prefersReducedMotion() ? 'auto' : 'smooth',
        block: 'center',
        inline: 'nearest',
      });
      el.classList.add(FIELD_HIGHLIGHT_CLASS);
      highlightedElRef.current = el;
      highlightTimerRef.current = window.setTimeout(() => {
        el.classList.remove(FIELD_HIGHLIGHT_CLASS);
        highlightTimerRef.current = null;
        highlightedElRef.current = null;
      }, 1800);
    };

    requestAnimationFrame(() => attempt(1));
  }, [jumpRequest, values.tlsEnable]);

  // Clear the highlight timer on unmount.
  useEffect(
    () => () => {
      if (highlightTimerRef.current !== null) clearTimeout(highlightTimerRef.current);
    },
    []
  );

  return { jumpToField };
}
