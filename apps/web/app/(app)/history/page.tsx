import { requireAccount } from "../../account-server";

export default async function HistoryPage() {
  await requireAccount("/history");
  return (
    <main className="account-main">
      <h1>History</h1>
      <p>Board dari meja aktif dan Team Match yang dapat kamu akses akan tersedia di sini.</p>
    </main>
  );
}
