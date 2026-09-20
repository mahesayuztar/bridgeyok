"use client";

import IssueNotice from "../issue-notice";

export default function WorkspaceError({ reset }: { reset: () => void }) {
  return (
    <main className="workspace-route-state">
      <IssueNotice
        issue={{
          kind: "server",
          title: "Workspace tidak dapat dibuka",
          detail: "Meja tetap terhubung. Coba muat workspace ini sekali lagi.",
          retryable: true,
          action: "retry",
          source: "browser",
        }}
        onAction={reset}
      />
    </main>
  );
}
