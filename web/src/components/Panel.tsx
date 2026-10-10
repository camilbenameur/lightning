import { useId, type ReactNode } from 'react';
import './Panel.css';

interface PanelProps {
  title: ReactNode;
  /** Controls shown on the right of the panel header. */
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
}

/** A titled region of the workspace (editor, stdin, output). */
export function Panel({ title, actions, className = '', children }: PanelProps) {
  const titleId = useId();
  return (
    <section className={`panel ${className}`} aria-labelledby={titleId}>
      <header className="panel__header">
        <h2 className="panel__title" id={titleId}>
          {title}
        </h2>
        {actions && <div className="panel__actions">{actions}</div>}
      </header>
      <div className="panel__body">{children}</div>
    </section>
  );
}
