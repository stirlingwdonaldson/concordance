"use client";

/** A plain download link: the API sends the CSV and the browser saves it. */
export function ExportLink({
  href,
  label = "Export CSV",
  title,
}: Readonly<{ href: string; label?: string; title?: string }>) {
  return (
    <a className="btn btn-quiet btn-sm" href={href} download title={title}>
      {label}
    </a>
  );
}
