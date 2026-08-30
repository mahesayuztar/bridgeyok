"use client";

import Link from "next/link";
import IssueNotice from "./issue-notice";

export default function AppError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="table-route-state">
      <IssueNotice
        issue={{
          kind: "server",
          title: "Tampilan perlu dimuat ulang",
          detail: "Keadaan meja tetap aman di server. Coba muat ulang tampilan atau kembali ke lobby.",
          retryable: true,
          action: "retry",
          source: "browser"
        }}
        onAction={() => reset()}
      />
      <Link className="text-link" href="/lobby">Kembali ke lobby</Link>
    </main>
  );
}
