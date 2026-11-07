import { TerminalView } from '@/components/terminal/TerminalView';
import styles from './LogsPage.module.scss';

export function LogsPage() {
  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Console Log</h1>
        <p className={styles.subtitle}>Live server console output</p>
      </div>

      <TerminalView />
    </div>
  );
}
