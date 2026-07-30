import type { ReactNode } from 'react';

export type OperationToolbarProps = {
  children: ReactNode;
  extra?: ReactNode;
  className?: string;
};

/** Inline command bar for page headers, table toolbars, and compact action groups. */
export default function OperationToolbar({ children, extra, className }: OperationToolbarProps) {
  return (
    <div className={['tm-operation-toolbar', className].filter(Boolean).join(' ')}>
      <div className="tm-operation-toolbar__main">{children}</div>
      {extra ? <div className="tm-operation-toolbar__extra">{extra}</div> : null}
    </div>
  );
}
