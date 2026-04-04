export function StatusBar({ message }: { message: string }) {
  if (!message) {
    return null;
  }

  return <section className="panel">{message}</section>;
}
