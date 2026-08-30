"use client";

import type { components } from "@bridgeyok/contracts/openapi";
import type { MutationCommandEnvelope } from "@bridgeyok/contracts/realtime";
import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import {
  createEmptyTableState,
  reduceTableState,
  type CommandName,
  type LiveTableProjection,
  type TableClientState
} from "./table-state";

type GuestCredentials = components["schemas"]["GuestCredentials"];
type CreateTableResponse = components["schemas"]["CreateTableResponse"];
type RealtimeTicket = components["schemas"]["RealtimeTicket"];
type Problem = components["schemas"]["Problem"];
type ConnectionState = "idle" | "connecting" | "syncing" | "connected" | "degraded" | "offline";

type StoredIdentity = Pick<GuestCredentials, "sessionId" | "nickname" | "deviceCredential">;
type StoredAccess = Pick<GuestCredentials, "accessToken" | "accessExpiresAt">;
type StoredTable = { tableId: string; inviteCode?: string };

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
const IDENTITY_KEY = "bridgeyok.identity.v1";
const ACCESS_KEY = "bridgeyok.access.v1";
const TABLE_KEY = "bridgeyok.table.v1";

const ERROR_MESSAGES: Record<string, string> = {
  access_revoked: "Akses ke meja ini sudah dicabut.",
  already_seated: "Kamu sudah duduk di kursi lain.",
  card_not_held: "Kartu itu tidak ada di tangan yang sedang dimainkan.",
  duplicate_request_mismatch: "Permintaan lama tidak cocok. Muat ulang keadaan meja.",
  declarer_controls_dummy: "Kartu dummy dimainkan oleh declarer.",
  forbidden: "Aksi ini tidak diizinkan untuk peranmu.",
  illegal_call: "Call itu belum legal pada urutan lelang ini.",
  invalid_command: "Perintah tidak cocok dengan keadaan meja saat ini.",
  must_follow_suit: "Kamu masih memiliki kartu dengan suit yang dipimpin.",
  not_your_turn: "Belum giliranmu.",
  owner_cannot_leave: "Pemilik perlu mengakhiri meja, bukan meninggalkannya.",
  owner_required: "Hanya pemilik yang dapat melakukan aksi ini.",
  seat_required: "Pilih kursi terlebih dahulu.",
  seat_taken: "Kursi itu baru saja ditempati pemain lain.",
  stale_controller: "Kendali kursi berpindah perangkat. Ambil alih kursi untuk melanjutkan.",
  state_changed: "Meja berubah lebih dulu. Keadaan terbaru sedang dimuat.",
  revision_conflict: "Meja berubah lebih dulu. Keadaan terbaru sedang dimuat.",
  table_full: "Meja sudah penuh.",
  table_locked: "Meja sedang dikunci pemilik.",
  table_not_ready: "Empat kursi harus terisi dan siap sebelum board dimulai.",
  validation_failed: "Periksa kembali isian atau pilihanmu."
};

class ApiError extends Error {
  code: string | undefined;

  constructor(message: string, code?: string) {
    super(message);
    this.code = code;
  }
}

function readStoredValue<T>(storage: Storage, key: string): T | null {
  try {
    const value = storage.getItem(key);
    return value === null ? null : (JSON.parse(value) as T);
  } catch {
    storage.removeItem(key);
    return null;
  }
}

function persistCredentials(credentials: GuestCredentials) {
  const identity: StoredIdentity = {
    sessionId: credentials.sessionId,
    nickname: credentials.nickname,
    deviceCredential: credentials.deviceCredential
  };
  const access: StoredAccess = {
    accessToken: credentials.accessToken,
    accessExpiresAt: credentials.accessExpiresAt
  };
  localStorage.setItem(IDENTITY_KEY, JSON.stringify(identity));
  sessionStorage.setItem(ACCESS_KEY, JSON.stringify(access));
}

function friendlyError(code: string | undefined, fallback: string) {
  return code === undefined ? fallback : (ERROR_MESSAGES[code.toLowerCase()] ?? fallback);
}

async function readProblem(response: Response, fallback: string): Promise<ApiError> {
  try {
    const problem = (await response.json()) as Problem;
    return new ApiError(friendlyError(problem.code, problem.title || fallback), problem.code);
  } catch {
    return new ApiError(fallback);
  }
}

async function requestJson<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(new URL(path, API_BASE_URL), init);
  if (!response.ok) {
    throw await readProblem(response, "Layanan belum dapat memproses permintaan.");
  }
  return (await response.json()) as T;
}

function isLiveTableProjection(value: unknown): value is LiveTableProjection {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const table = value as Record<string, unknown>;
  return (
    typeof table.tableId === "string" &&
    ["WAITING", "ACTIVE", "BETWEEN_BOARDS", "FINISHED"].includes(String(table.state)) &&
    typeof table.revision === "number" &&
    typeof table.lastSeq === "number" &&
    typeof table.viewerParticipantId === "string" &&
    Array.isArray(table.participants) &&
    table.seats !== null &&
    typeof table.seats === "object"
  );
}

function socketUrl(ticket: string) {
  const url = new URL("/v1/ws", API_BASE_URL);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", ticket);
  return url;
}

export type TableSession = {
  initializing: boolean;
  busy: boolean;
  nickname: string | null;
  connectionState: ConnectionState;
  inviteCode: string | null;
  tableState: TableClientState;
  createIdentity: (nickname: string) => Promise<void>;
  logout: () => Promise<void>;
  createTable: () => Promise<void>;
  joinTable: (inviteCode: string) => Promise<void>;
  leaveTable: () => Promise<void>;
  reconnect: () => void;
  sendCommand: (name: CommandName, payload?: Record<string, unknown>) => void;
};

export function useTableSession(): TableSession {
  const [tableState, dispatch] = useReducer(reduceTableState, undefined, createEmptyTableState);
  const [initializing, setInitializing] = useState(true);
  const [busy, setBusy] = useState(false);
  const [nickname, setNickname] = useState<string | null>(null);
  const [inviteCode, setInviteCode] = useState<string | null>(null);
  const [connectionState, setConnectionState] = useState<ConnectionState>("idle");
  const credentialsRef = useRef<GuestCredentials | null>(null);
  const tableStateRef = useRef(tableState);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const connectionGenerationRef = useRef(0);
  const refreshPromiseRef = useRef<Promise<GuestCredentials> | null>(null);

  useEffect(() => {
    tableStateRef.current = tableState;
  }, [tableState]);

  const refreshCredentials = useCallback(async (identity?: StoredIdentity) => {
    if (refreshPromiseRef.current !== null) {
      return refreshPromiseRef.current;
    }
    const storedIdentity = identity ?? readStoredValue<StoredIdentity>(localStorage, IDENTITY_KEY);
    if (storedIdentity === null) {
      throw new ApiError("Sesi tamu tidak ditemukan.");
    }
    const promise = requestJson<GuestCredentials>(`/v1/guest-sessions/${encodeURIComponent(storedIdentity.sessionId)}/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ deviceCredential: storedIdentity.deviceCredential })
    });
    refreshPromiseRef.current = promise;
    try {
      const credentials = await promise;
      credentialsRef.current = credentials;
      persistCredentials(credentials);
      setNickname(credentials.nickname);
      return credentials;
    } finally {
      refreshPromiseRef.current = null;
    }
  }, []);

  const ensureAccessToken = useCallback(async () => {
    const credentials = credentialsRef.current;
    if (credentials !== null && Date.parse(credentials.accessExpiresAt) > Date.now() + 30_000) {
      return credentials.accessToken;
    }
    return (await refreshCredentials()).accessToken;
  }, [refreshCredentials]);

  const authenticatedRequest = useCallback(
    async <T,>(path: string, init: RequestInit = {}): Promise<T> => {
      let accessToken = await ensureAccessToken();
      let response = await fetch(new URL(path, API_BASE_URL), {
        ...init,
        headers: { ...init.headers, Authorization: `Bearer ${accessToken}` }
      });
      if (response.status === 401) {
        credentialsRef.current = null;
        accessToken = (await refreshCredentials()).accessToken;
        response = await fetch(new URL(path, API_BASE_URL), {
          ...init,
          headers: { ...init.headers, Authorization: `Bearer ${accessToken}` }
        });
      }
      if (!response.ok) {
        throw await readProblem(response, "Permintaan ke meja gagal.");
      }
      if (response.status === 204) {
        return undefined as T;
      }
      return (await response.json()) as T;
    },
    [ensureAccessToken, refreshCredentials]
  );

  const stopConnection = useCallback(() => {
    connectionGenerationRef.current += 1;
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    socketRef.current?.close(1000, "table changed");
    socketRef.current = null;
  }, []);

  const clearTable = useCallback(() => {
    stopConnection();
    localStorage.removeItem(TABLE_KEY);
    setInviteCode(null);
    setConnectionState("idle");
    dispatch({ type: "clear" });
  }, [stopConnection]);

  const openConnection = useCallback(
    async function connect(tableId: string, generation: number, attempt: number): Promise<void> {
      if (connectionGenerationRef.current !== generation) {
        return;
      }
      setConnectionState(navigator.onLine ? "connecting" : "offline");
      if (!navigator.onLine) {
        return;
      }
      try {
        const realtimeTicket = await authenticatedRequest<RealtimeTicket>("/v1/realtime/tickets", { method: "POST" });
        if (connectionGenerationRef.current !== generation) {
          return;
        }
        const socket = new WebSocket(socketUrl(realtimeTicket.ticket));
        socketRef.current = socket;
        socket.onopen = () => {
          if (connectionGenerationRef.current !== generation) {
            socket.close(1000, "stale connection");
            return;
          }
          setConnectionState("syncing");
          const lastSeenSeq = tableStateRef.current.activeTableId === tableId ? tableStateRef.current.lastSeenSeq : 0;
          socket.send(
            JSON.stringify({
              v: 1,
              kind: "command",
              name: lastSeenSeq > 0 ? "table.resume" : "table.subscribe",
              request_id: `req_${crypto.randomUUID().replaceAll("-", "")}`,
              table_id: tableId,
              payload: { last_seen_seq: lastSeenSeq }
            })
          );
        };
        socket.onmessage = (message) => {
          let envelope: Record<string, unknown>;
          try {
            const parsed = JSON.parse(String(message.data)) as unknown;
            if (parsed === null || typeof parsed !== "object") {
              return;
            }
            envelope = parsed as Record<string, unknown>;
          } catch {
            return;
          }
          if (envelope.table_id !== undefined && envelope.table_id !== tableId) {
            return;
          }
          if (envelope.kind === "snapshot" && isLiveTableProjection(envelope.payload)) {
            dispatch({ type: "snapshot", tableId, seq: Number(envelope.seq), table: envelope.payload });
            setConnectionState("connected");
          } else if (envelope.kind === "event") {
            const payload = envelope.payload as Record<string, unknown> | undefined;
            if (payload !== undefined && isLiveTableProjection(payload.table)) {
              dispatch({ type: "event", tableId, seq: Number(envelope.seq), table: payload.table });
              setConnectionState("connected");
            }
          } else if (envelope.kind === "error") {
            const code = typeof envelope.code === "string" ? envelope.code : undefined;
            const messageText = friendlyError(code, "Aksi ditolak oleh meja.");
            if (typeof envelope.request_id === "string") {
              dispatch({ type: "settled", requestId: envelope.request_id, message: messageText });
            } else {
              dispatch({ type: "message", message: messageText });
            }
          } else if (envelope.kind === "control" && envelope.name === "table.access_revoked") {
            clearTable();
          } else if (envelope.kind === "control" && envelope.name === "server.draining") {
            setConnectionState("degraded");
            socket.close(1012, "server draining");
          }
        };
        socket.onclose = (event) => {
          if (connectionGenerationRef.current !== generation || event.code === 1000) {
            return;
          }
          socketRef.current = null;
          setConnectionState(navigator.onLine ? "degraded" : "offline");
          dispatch({ type: "connectionLost", message: "Koneksi terputus. Perintah yang belum dijawab tidak dikirim ulang." });
          const delay = Math.min(10_000, 500 * 2 ** attempt) + Math.floor(Math.random() * 300);
          reconnectTimerRef.current = setTimeout(() => void connect(tableId, generation, attempt + 1), delay);
        };
      } catch (error) {
        if (connectionGenerationRef.current !== generation) {
          return;
        }
        setConnectionState(navigator.onLine ? "degraded" : "offline");
        dispatch({ type: "connectionLost", message: error instanceof Error ? error.message : "Koneksi meja gagal." });
        const delay = Math.min(10_000, 500 * 2 ** attempt) + Math.floor(Math.random() * 300);
        reconnectTimerRef.current = setTimeout(() => void connect(tableId, generation, attempt + 1), delay);
      }
    },
    [authenticatedRequest, clearTable]
  );

  const beginConnection = useCallback(
    (tableId: string) => {
      stopConnection();
      const generation = connectionGenerationRef.current;
      void openConnection(tableId, generation, 0);
    },
    [openConnection, stopConnection]
  );

  useEffect(() => {
    let active = true;
    async function restoreSession() {
      const identity = readStoredValue<StoredIdentity>(localStorage, IDENTITY_KEY);
      if (identity === null) {
        setInitializing(false);
        return;
      }
      try {
        const access = readStoredValue<StoredAccess>(sessionStorage, ACCESS_KEY);
        if (access !== null && Date.parse(access.accessExpiresAt) > Date.now() + 30_000) {
          credentialsRef.current = { ...identity, ...access };
          setNickname(identity.nickname);
        } else {
          await refreshCredentials(identity);
        }
        const storedTable = readStoredValue<StoredTable>(localStorage, TABLE_KEY);
        if (storedTable !== null && active) {
          const table = await authenticatedRequest<unknown>(`/v1/tables/${encodeURIComponent(storedTable.tableId)}`);
          if (isLiveTableProjection(table)) {
            setInviteCode(storedTable.inviteCode ?? null);
            dispatch({ type: "enter", table });
            beginConnection(table.tableId);
          } else {
            localStorage.removeItem(TABLE_KEY);
          }
        }
      } catch (error) {
        if (error instanceof ApiError && error.code === "session_invalid") {
          localStorage.removeItem(IDENTITY_KEY);
          sessionStorage.removeItem(ACCESS_KEY);
          setNickname(null);
        } else {
          dispatch({ type: "message", message: error instanceof Error ? error.message : "Sesi gagal dipulihkan." });
        }
      } finally {
        if (active) {
          setInitializing(false);
        }
      }
    }
    void restoreSession();
    return () => {
      active = false;
      stopConnection();
    };
  }, [authenticatedRequest, beginConnection, refreshCredentials, stopConnection]);

  useEffect(() => {
    function handleOnline() {
      const tableId = tableStateRef.current.activeTableId;
      if (tableId !== null) {
        beginConnection(tableId);
      }
    }
    function handleOffline() {
      setConnectionState("offline");
      socketRef.current?.close();
    }
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, [beginConnection]);

  const createIdentity = useCallback(async (newNickname: string) => {
    setBusy(true);
    try {
      const credentials = await requestJson<GuestCredentials>("/v1/guest-sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname: newNickname.trim() })
      });
      credentialsRef.current = credentials;
      persistCredentials(credentials);
      setNickname(credentials.nickname);
      dispatch({ type: "message", message: null });
    } catch (error) {
      dispatch({ type: "message", message: error instanceof Error ? error.message : "Nama tamu gagal dibuat." });
    } finally {
      setBusy(false);
    }
  }, []);

  const logout = useCallback(async () => {
    setBusy(true);
    clearTable();
    try {
      if (credentialsRef.current !== null) {
        await authenticatedRequest<void>(`/v1/guest-sessions/${encodeURIComponent(credentialsRef.current.sessionId)}`, { method: "DELETE" });
      }
    } catch {
    } finally {
      credentialsRef.current = null;
      localStorage.removeItem(IDENTITY_KEY);
      sessionStorage.removeItem(ACCESS_KEY);
      setNickname(null);
      setBusy(false);
    }
  }, [authenticatedRequest, clearTable]);

  const createTable = useCallback(async () => {
    setBusy(true);
    try {
      const created = await authenticatedRequest<CreateTableResponse>("/v1/tables", { method: "POST" });
      if (!isLiveTableProjection(created.table)) {
        throw new ApiError("Data meja tidak dikenali.");
      }
      clearTable();
      localStorage.setItem(TABLE_KEY, JSON.stringify({ tableId: created.table.tableId, inviteCode: created.inviteCode }));
      setInviteCode(created.inviteCode);
      dispatch({ type: "enter", table: created.table });
      beginConnection(created.table.tableId);
    } catch (error) {
      dispatch({ type: "message", message: error instanceof Error ? error.message : "Meja gagal dibuat." });
    } finally {
      setBusy(false);
    }
  }, [authenticatedRequest, beginConnection, clearTable]);

  const joinTable = useCallback(
    async (rawInviteCode: string) => {
      const normalizedInviteCode = rawInviteCode.trim().toUpperCase();
      setBusy(true);
      try {
        const table = await authenticatedRequest<unknown>(`/v1/tables/${encodeURIComponent(normalizedInviteCode)}/join`, { method: "POST" });
        if (!isLiveTableProjection(table)) {
          throw new ApiError("Data meja tidak dikenali.");
        }
        clearTable();
        localStorage.setItem(TABLE_KEY, JSON.stringify({ tableId: table.tableId, inviteCode: normalizedInviteCode }));
        setInviteCode(normalizedInviteCode);
        dispatch({ type: "enter", table });
        beginConnection(table.tableId);
      } catch (error) {
        dispatch({ type: "message", message: error instanceof Error ? error.message : "Meja tidak dapat dimasuki." });
      } finally {
        setBusy(false);
      }
    },
    [authenticatedRequest, beginConnection, clearTable]
  );

  const leaveTable = useCallback(async () => {
    const table = tableStateRef.current.table;
    if (table === null) {
      return;
    }
    setBusy(true);
    try {
      if (table.state === "FINISHED") {
        clearTable();
      } else if (table.state === "WAITING") {
        await authenticatedRequest<void>(`/v1/tables/${encodeURIComponent(table.tableId)}`, { method: "DELETE" });
        clearTable();
      } else {
        dispatch({ type: "message", message: "Akhiri permainan dari kontrol meja sebelum meninggalkannya." });
      }
    } catch (error) {
      dispatch({ type: "message", message: error instanceof Error ? error.message : "Belum dapat meninggalkan meja." });
    } finally {
      setBusy(false);
    }
  }, [authenticatedRequest, clearTable]);

  const reconnect = useCallback(() => {
    const tableId = tableStateRef.current.activeTableId;
    if (tableId !== null) {
      beginConnection(tableId);
    }
  }, [beginConnection]);

  const sendCommand = useCallback((name: CommandName, payload: Record<string, unknown> = {}) => {
    const socket = socketRef.current;
    const table = tableStateRef.current.table;
    if (socket === null || socket.readyState !== WebSocket.OPEN || table === null) {
      dispatch({ type: "message", message: "Tunggu hingga koneksi meja pulih sebelum mencoba lagi." });
      return;
    }
    const requestId = `req_${crypto.randomUUID().replaceAll("-", "")}`;
    const controllerEpoch = table.viewerSeat === undefined ? undefined : table.seats[table.viewerSeat]?.controllerEpoch;
    const command: MutationCommandEnvelope = {
      v: 1,
      kind: "command",
      name,
      request_id: requestId,
      table_id: table.tableId,
      expected_revision: table.revision,
      payload,
      ...(controllerEpoch === undefined ? {} : { controller_epoch: controllerEpoch })
    };
    dispatch({ type: "pending", requestId, commandName: name });
    try {
      socket.send(JSON.stringify(command));
    } catch {
      dispatch({ type: "settled", requestId, message: "Perintah tidak terkirim. Silakan coba kembali." });
    }
  }, []);

  return {
    initializing,
    busy,
    nickname,
    connectionState,
    inviteCode,
    tableState,
    createIdentity,
    logout,
    createTable,
    joinTable,
    leaveTable,
    reconnect,
    sendCommand
  };
}
