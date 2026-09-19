"use client";

import IssueNotice from "./issue-notice";

export default function AppError() {
  return (
    <main className="table-route-state">
      <IssueNotice
        issue={{
          kind: "server",
          title: "Tampilan perlu dimuat ulang",
          detail: "Coba muat ulang tampilan. Jika masalah berulang, kembali ke lobby untuk keluar dari proses pemulihan meja.",
          retryable: true,
          action: "retry",
          source: "browser"
        }}
        onAction={() => window.location.reload()}
      />
      <a
        className="text-link"
        href="/lobby"
        onClick={() => {
          try {
            localStorage.removeItem("bridgeyok.table.v1");
          } catch {
          }
        }}
      >
        Kembali ke lobby
      </a>
    </main>
  );
}
