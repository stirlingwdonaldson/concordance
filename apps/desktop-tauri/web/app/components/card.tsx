import type { ReactNode } from "react";

export function Card({
  title,
  hint,
  actions,
  className = "",
  children,
}: Readonly<{
  title: ReactNode;
  hint?: ReactNode;
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
}>) {
  return (
    <section className={`card ${className}`}>
      <header className="card-head">
        <div>
          <h2>{title}</h2>
          {hint && <p className="card-hint">{hint}</p>}
        </div>
        {actions && <div className="card-actions">{actions}</div>}
      </header>
      <div className="card-body">{children}</div>
    </section>
  );
}
