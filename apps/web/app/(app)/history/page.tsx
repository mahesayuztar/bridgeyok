import { requireAccount } from "../../account-server";
import { HistoryWorkspace } from "../../history-workspace";

export default async function HistoryPage() {
  await requireAccount("/history");
  return (
    <main className="account-main history-page">
      <HistoryWorkspace />
    </main>
  );
}
