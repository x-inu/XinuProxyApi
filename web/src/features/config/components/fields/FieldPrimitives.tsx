import type { ReactNode } from 'react';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import { configFieldDomId } from '../../searchIndex';
import styles from './Field.module.scss';


export const FIELD_HIGHLIGHT_CLASS: string = styles.fieldHighlightActive;


export const FIELDS_ROOT_CLASS: string = styles.fieldsRoot;

export type ToggleRowProps = {
  title: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (value: boolean) => void;
};

export function ToggleRow({ title, description, checked, disabled, onChange }: ToggleRowProps) {
  return (
    <div className={styles.toggleRow}>
      <div className={styles.toggleCopy}>
        <div className={styles.toggleTitle}>{title}</div>
        {description ? <div className={styles.toggleDescription}>{description}</div> : null}
      </div>
      <ToggleSwitch checked={checked} onChange={onChange} disabled={disabled} ariaLabel={title} />
    </div>
  );
}

export function FieldGrid({ children }: { children: ReactNode }) {
  return <div className={styles.fieldGrid}>{children}</div>;
}

export function FieldStack({ children }: { children: ReactNode }) {
  return <div className={styles.fieldStack}>{children}</div>;
}

export function Divider() {
  return <div className={styles.divider} />;
}

// Stable, stateless anchor around a searchable field. Search jumps target its DOM id
// (see searchIndex.ts) and the highlight pulse is applied to it imperatively.

export function FieldAnchor({
  fieldId,
  wide = false,
  children,
}: {
  fieldId: string;
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      id={configFieldDomId(fieldId)}
      className={`${styles.fieldAnchor} ${wide ? styles.fieldAnchorWide : ''}`}
    >
      {children}
    </div>
  );
}


export function FieldGroup({
  title,
  description,
  children,
}: {
  title?: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.group}>
      {title ? (
        <div className={styles.groupHeader}>
          <h3 className={styles.groupTitle}>{title}</h3>
          {description ? <p className={styles.groupDescription}>{description}</p> : null}
        </div>
      ) : null}
      {children}
    </div>
  );
}


export function FieldGroupHeading({ title }: { title: string }) {
  return (
    <div className={styles.groupHeader}>
      <h3 className={styles.groupTitle}>{title}</h3>
    </div>
  );
}

export function FieldShell({
  label,
  labelId,
  htmlFor,
  hint,
  hintId,
  error,
  errorId,
  children,
}: {
  label: string;
  labelId?: string;
  htmlFor?: string;
  hint?: string;
  hintId?: string;
  error?: string;
  errorId?: string;
  children: ReactNode;
}) {
  return (
    <div className={styles.fieldShell}>
      <label id={labelId} htmlFor={htmlFor} className={styles.fieldLabel}>
        {label}
      </label>
      {children}
      {error ? (
        <div id={errorId} className="error-box">
          {error}
        </div>
      ) : null}
      {hint ? (
        <div id={hintId} className={styles.fieldHint}>
          {hint}
        </div>
      ) : null}
    </div>
  );
}


export function FieldHint({ id, children }: { id?: string; children: ReactNode }) {
  return (
    <div id={id} className={styles.fieldHint}>
      {children}
    </div>
  );
}


export function FieldControl({ children }: { children: ReactNode }) {
  return <div className={styles.fieldControl}>{children}</div>;
}


export function InlinePill({ children }: { children: ReactNode }) {
  return <span className={styles.inlinePill}>{children}</span>;
}
