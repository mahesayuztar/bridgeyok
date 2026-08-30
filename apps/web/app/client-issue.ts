export type ClientIssue = {
  kind: "notFound" | "unavailable" | "full" | "locked" | "offline" | "network" | "timeout" | "session" | "conflict" | "server" | "validation";
  title: string;
  detail: string;
  retryable: boolean;
  action?: "retry" | "editInvite" | "backToLobby" | "signInAgain" | "resync" | "takeover";
  source: "rest" | "websocket" | "browser";
};

type IssueInput = {
  code?: string;
  status?: number;
  retryable?: boolean;
  source: ClientIssue["source"];
};

export function issueFromServer({ code, status, retryable = false, source }: IssueInput): ClientIssue {
  switch (code?.toUpperCase()) {
    case "TABLE_NOT_FOUND":
      return { kind: "notFound", title: "Meja tidak ditemukan", detail: "Kode mungkin salah atau meja sudah dihapus.", retryable: false, action: "editInvite", source };
    case "TABLE_UNAVAILABLE":
      return { kind: "unavailable", title: "Meja sudah tidak tersedia", detail: "Meja mungkin sudah dimulai, selesai, atau undangannya tidak berlaku.", retryable: false, action: "backToLobby", source };
    case "TABLE_FULL":
      return { kind: "full", title: "Meja sudah penuh", detail: "Empat pemain sudah bergabung. Gunakan kode meja lain untuk bermain.", retryable: false, action: "backToLobby", source };
    case "TABLE_LOCKED":
      return { kind: "locked", title: "Meja sedang dikunci", detail: "Pemilik belum membuka meja untuk pemain baru.", retryable: true, action: "retry", source };
    case "STATE_CHANGED":
    case "REVISION_CONFLICT":
      return { kind: "conflict", title: "Meja sudah berubah", detail: "Keadaan terbaru perlu diselaraskan sebelum aksi berikutnya.", retryable: true, action: "resync", source };
    case "STALE_CONTROLLER":
      return { kind: "conflict", title: "Kendali ada di perangkat lain", detail: "Selaraskan meja, lalu ambil alih kendali dari perangkat ini.", retryable: true, action: "resync", source };
    case "SESSION_INACTIVE":
    case "SESSION_INVALID":
    case "UNAUTHORIZED":
      return { kind: "session", title: "Sesi tamu sudah berakhir", detail: "Masuk kembali sebelum membuka meja. Data privat meja sudah dibersihkan dari tampilan.", retryable: false, action: "signInAgain", source };
    case "ILLEGAL_CALL":
      return { kind: "validation", title: "Call belum legal", detail: "Pilih call lain yang tersedia pada urutan lelang ini.", retryable: false, source };
    case "CARD_NOT_HELD":
      return { kind: "validation", title: "Kartu tidak dapat dimainkan", detail: "Kartu itu tidak ada di tangan yang sedang dikendalikan.", retryable: false, source };
    case "MUST_FOLLOW_SUIT":
      return { kind: "validation", title: "Harus mengikuti suit", detail: "Kamu masih memiliki kartu dengan suit yang dipimpin.", retryable: false, source };
    case "NOT_YOUR_TURN":
      return { kind: "validation", title: "Belum giliranmu", detail: "Tunggu pemain yang sedang mendapat giliran.", retryable: false, source };
    case "SEAT_TAKEN":
      return { kind: "validation", title: "Kursi baru saja terisi", detail: "Pilih kursi kosong lain setelah keadaan meja diperbarui.", retryable: false, source };
    case "SERVICE_UNAVAILABLE":
    case "SERVER_BUSY":
    case "INTERNAL_ERROR":
      return { kind: "server", title: "Layanan meja sedang bermasalah", detail: "Aksi belum dapat dipastikan berhasil. Selaraskan meja sebelum mencoba lagi.", retryable: true, action: "retry", source };
  }
  if (status === 401 || status === 403) {
    return { kind: "session", title: "Sesi tamu tidak dapat digunakan", detail: "Masuk kembali untuk melanjutkan dengan sesi baru.", retryable: false, action: "signInAgain", source };
  }
  if (status !== undefined && status >= 500) {
    return { kind: "server", title: "Layanan meja sedang bermasalah", detail: "Server belum dapat memproses permintaan ini.", retryable: true, action: "retry", source };
  }
  return { kind: "validation", title: "Aksi belum dapat dilakukan", detail: "Periksa kembali pilihanmu dan keadaan meja saat ini.", retryable, source };
}

export function issueFromFailure(error: unknown, source: ClientIssue["source"] = "browser"): ClientIssue {
  if (typeof navigator !== "undefined" && !navigator.onLine) {
    return { kind: "offline", title: "Kamu sedang offline", detail: "Keadaan terakhir tetap terlihat. Aksi akan aktif lagi setelah koneksi pulih.", retryable: true, action: "retry", source: "browser" };
  }
  if (error instanceof DOMException && (error.name === "TimeoutError" || error.name === "AbortError")) {
    return { kind: "timeout", title: "Server terlalu lama merespons", detail: "Hasil permintaan belum dapat dipastikan. Selaraskan keadaan sebelum mengirim aksi baru.", retryable: true, action: "retry", source };
  }
  return { kind: "network", title: "Tidak dapat menghubungi server", detail: "Data meja belum dapat diperiksa. Periksa koneksi lalu coba lagi.", retryable: true, action: "retry", source };
}
