"use client";

import { useEffect, useRef } from "react";
import type { ClientIssue } from "./client-issue";

const ACTION_LABELS: Record<NonNullable<ClientIssue["action"]>, string> = {
  retry: "Coba lagi",
  editInvite: "Periksa kode",
  backToLobby: "Kembali ke lobby",
  signInAgain: "Masuk kembali",
  resync: "Selaraskan meja"
};

export default function IssueNotice({ issue, compact = false, onAction, onDismiss }: {
  issue: ClientIssue;
  compact?: boolean;
  onAction?: (action: NonNullable<ClientIssue["action"]>) => void;
  onDismiss?: () => void;
}) {
  const noticeRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    noticeRef.current?.focus();
  }, [issue]);

  return (
    <div className={compact ? "issue-notice issue-notice-compact" : "issue-notice"} data-kind={issue.kind} role="alert" tabIndex={-1} ref={noticeRef}>
      <div>
        <strong>{issue.title}</strong>
        <p>{issue.detail}</p>
      </div>
      <div className="issue-actions">
        {issue.action === undefined || onAction === undefined ? null : (
          <button type="button" onClick={() => onAction(issue.action!)}>{ACTION_LABELS[issue.action]}</button>
        )}
        {onDismiss === undefined ? null : <button className="issue-dismiss" type="button" onClick={onDismiss} aria-label="Tutup pemberitahuan">×</button>}
      </div>
    </div>
  );
}
