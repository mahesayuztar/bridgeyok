import { requireAccount } from "../../account-server";
import { HistoryWorkspace } from "../../history-workspace";

export default async function HistoryPage() {
  await requireAccount("/history");
  return (
    <main className="account-main history-page">
      <h1>History</h1>
      <p className="history-page-subtitle">Riwayat sesi dan hasil board.</p>
      <HistoryWorkspace />
    </main>
  );
}
