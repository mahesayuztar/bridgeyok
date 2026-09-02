"use client";

import type { components } from "@bridgeyok/contracts/openapi";
import type { MutationCommandEnvelope } from "@bridgeyok/contracts/realtime";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { issueFromFailure, issueFromServer, type ClientIssue } from "./client-issue";
import { canSendTableCommand } from "./gameplay-capabilities";
import { createRequestId } from "./request-id";
import { normalizeLiveTableProjection, normalizeParticipantPresence, normalizePresenceSnapshot } from "./table-projection";
import {
  createEmptyTableState,
  projectedTableState,
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
const REALTIME_CONNECTION_ROTATION_MS = 285_000;
const IDENTITY_KEY = "bridgeyok.identity.v1";
const ACCESS_KEY = "bridgeyok.access.v1";
const TABLE_KEY = "bridgeyok.table.v1";

class ApiError extends Error {
  code: string | undefined;
  issue: ClientIssue;

  constructor(issue: ClientIssue, code?: string) {
    super(issue.title);
    this.code = code;
    this.issue = issue;
  }
}

function browserStorage(kind: "local" | "session"): Storage | null {
  try {
    return kind === "local" ? window.localStorage : window.sessionStorage;
  } catch {
    return null;
  }
}

function readStoredValue<T>(storage: Storage | null, key: string): T | null {
  if (storage === null) {
    return null;
  }
  try {
    const value = storage.getItem(key);
    return value === null ? null : (JSON.parse(value) as T);
  } catch {
    try {
      storage.removeItem(key);
    } catch {
    }
    return null;
  }
}

function writeStoredValue(storage: Storage | null, key: string, value: unknown) {
  try {
    storage?.setItem(key, JSON.stringify(value));
  } catch {
  }
}

function removeStoredValue(storage: Storage | null, key: string) {
  try {
    storage?.removeItem(key);
  } catch {
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
  writeStoredValue(browserStorage("local"), IDENTITY_KEY, identity);
  writeStoredValue(browserStorage("session"), ACCESS_KEY, access);
}

async function readProblem(response: Response): Promise<ApiError> {
  try {
    const problem = (await response.json()) as Problem;
    return new ApiError(issueFromServer({
      status: response.status,
      source: "rest",
      ...(problem.code === undefined ? {} : { code: problem.code }),
      ...(problem.retryable === undefined ? {} : { retryable: problem.retryable })
    }), problem.code);
  } catch {
    return new ApiError(issueFromServer({ status: response.status, source: "rest" }));
  }
}

async function requestJson<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response;
  try {
    response = await fetch(new URL(path, API_BASE_URL), { ...init, signal: init.signal ?? AbortSignal.timeout(8000) });
  } catch (error) {
    throw new ApiError(issueFromFailure(error, "rest"));
  }
  if (!response.ok) {
    throw await readProblem(response);
  }
  return (await response.json()) as T;
}

function socketUrl(ticket: string) {
  const url = new URL("/v1/ws", API_BASE_URL);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", ticket);
  return url;
}

function issueForError(error: unknown): ClientIssue {
  return error instanceof ApiError ? error.issue : issueFromFailure(error, "rest");
}

export type TableSession = {
  initializing: boolean;
  busy: boolean;
  nickname: string | null;
  connectionState: ConnectionState;
  inviteCode: string | null;
  tableState: TableClientState;
  projectedTable: LiveTableProjection | null;
  createIdentity: (nickname: string) => Promise<boolean>;
  logout: () => Promise<void>;
  createTable: () => Promise<string | null>;
  joinTable: (inviteCode: string) => Promise<string | null>;
  openTable: (tableId: string) => Promise<boolean>;
  leaveTable: () => Promise<void>;
  reconnect: () => void;
  resync: () => void;
  dismissIssue: () => void;
  dismissNotice: () => void;
  canSendCommand: (name: CommandName, payload?: Record<string, unknown>) => boolean;
  sendCommand: (name: CommandName, payload?: Record<string, unknown>) => void;
};

export function useTableSession({ restoreTable = true }: { restoreTable?: boolean } = {}): TableSession {
  const [tableState, dispatch] = useReducer(reduceTableState, undefined, createEmptyTableState);
  const projectedTable = useMemo(() => projectedTableState(tableState), [tableState]);
  const [initializing, setInitializing] = useState(restoreTable);
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
  const beginConnectionRef = useRef<((tableId: string) => void) | null>(null);
  const automaticTakeoverRevisionRef = useRef<number | null>(null);

  useEffect(() => {
    tableStateRef.current = tableState;
  }, [tableState]);

  const refreshCredentials = useCallback(async (identity?: StoredIdentity) => {
    if (refreshPromiseRef.current !== null) {
      return refreshPromiseRef.current;
    }
    const storedIdentity = identity ?? readStoredValue<StoredIdentity>(browserStorage("local"), IDENTITY_KEY);
    if (storedIdentity === null) {
      throw new ApiError(issueFromServer({ code: "SESSION_INVALID", source: "rest" }), "SESSION_INVALID");
    }
    const promise = requestJson<GuestCredentials>("/v1/guest-sessions/refresh", {
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
      let response: Response;
      try {
        response = await fetch(new URL(path, API_BASE_URL), {
          ...init,
          headers: { ...init.headers, Authorization: `Bearer ${accessToken}` },
          signal: init.signal ?? AbortSignal.timeout(8000)
        });
      } catch (error) {
        throw new ApiError(issueFromFailure(error, "rest"));
      }
      if (response.status === 401) {
        credentialsRef.current = null;
        accessToken = (await refreshCredentials()).accessToken;
        try {
          response = await fetch(new URL(path, API_BASE_URL), {
            ...init,
            headers: { ...init.headers, Authorization: `Bearer ${accessToken}` },
            signal: init.signal ?? AbortSignal.timeout(8000)
          });
        } catch (error) {
          throw new ApiError(issueFromFailure(error, "rest"));
        }
      }
      if (!response.ok) {
        throw await readProblem(response);
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
    removeStoredValue(browserStorage("local"), TABLE_KEY);
    setInviteCode(null);
    setConnectionState("idle");
    dispatch({ type: "clear" });
  }, [stopConnection]);

  const requestResync = useCallback((tableId: string) => {
    const socket = socketRef.current;
    if (socket === null || socket.readyState !== WebSocket.OPEN) {
      beginConnectionRef.current?.(tableId);
      return;
    }
    setConnectionState("syncing");
    socket.send(JSON.stringify({
      v: 1,
      kind: "command",
      name: "table.resume",
      request_id: createRequestId(),
      table_id: tableId,
      payload: { last_seen_seq: tableStateRef.current.lastSeenSeq }
    }));
  }, []);

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
        let plannedDisconnect = false;
        let rotationTimer: ReturnType<typeof setTimeout> | null = null;
        socketRef.current = socket;
        socket.onopen = () => {
          if (connectionGenerationRef.current !== generation) {
            socket.close(1000, "stale connection");
            return;
          }
          automaticTakeoverRevisionRef.current = null;
          dispatch({ type: "controllerSyncStarted" });
          setConnectionState("syncing");
          rotationTimer = setTimeout(() => {
            plannedDisconnect = true;
            setConnectionState("syncing");
            socket.close(4000, "connection rotation");
          }, REALTIME_CONNECTION_ROTATION_MS);
          const lastSeenSeq = tableStateRef.current.activeTableId === tableId ? tableStateRef.current.lastSeenSeq : 0;
          socket.send(
            JSON.stringify({
              v: 1,
              kind: "command",
              name: lastSeenSeq > 0 ? "table.resume" : "table.subscribe",
              request_id: createRequestId(),
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
          if (envelope.kind === "snapshot") {
            const projectedTable = normalizeLiveTableProjection(envelope.payload);
            if (projectedTable === null || typeof envelope.seq !== "number" || !Number.isFinite(envelope.seq)) {
              setConnectionState("degraded");
              dispatch({ type: "issue", issue: issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "websocket" }) });
              return;
            }
            dispatch({ type: "snapshot", tableId, seq: envelope.seq, table: projectedTable });
            setConnectionState("connected");
          } else if (envelope.kind === "event") {
            const payload = envelope.payload !== null && typeof envelope.payload === "object" && !Array.isArray(envelope.payload)
              ? envelope.payload as Record<string, unknown>
              : null;
            const eventTable = normalizeLiveTableProjection(payload?.table);
            if (payload === null || eventTable === null || typeof envelope.seq !== "number" || !Number.isFinite(envelope.seq)) {
              setConnectionState("degraded");
              dispatch({ type: "issue", issue: issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "websocket" }) });
              return;
            }
            const eventType = typeof payload.eventType === "string" ? payload.eventType : undefined;
            dispatch({
              type: "event",
              tableId,
              seq: envelope.seq,
              table: eventTable,
              ...(eventType === undefined ? {} : { eventType })
            });
            setConnectionState("connected");
          } else if (
            envelope.kind === "ack" &&
            typeof envelope.request_id === "string" &&
            typeof envelope.revision === "number" &&
            Number.isFinite(envelope.revision) &&
            typeof envelope.seq === "number" &&
            Number.isFinite(envelope.seq)
          ) {
            dispatch({
              type: "ack",
              requestId: envelope.request_id,
              revision: envelope.revision,
              seq: envelope.seq,
            });
          } else if (envelope.kind === "error") {
            const code = typeof envelope.code === "string" ? envelope.code : undefined;
            const issue = issueFromServer({
              retryable: envelope.retryable === true,
              source: "websocket",
              ...(code === undefined ? {} : { code })
            });
            if (code === "STALE_CONTROLLER" || code === "STATE_CHANGED" || code === "REVISION_CONFLICT") {
              dispatch({ type: "conflict", issue });
              requestResync(tableId);
            } else if (typeof envelope.request_id === "string") {
              dispatch({ type: "settled", requestId: envelope.request_id, issue });
            } else {
              dispatch({ type: "issue", issue });
            }
          } else if (envelope.kind === "control" && envelope.name === "presence.snapshot") {
            const participants = normalizePresenceSnapshot(envelope.payload);
            if (participants === null) {
              setConnectionState("degraded");
              dispatch({ type: "issue", issue: issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "websocket" }) });
              return;
            }
            dispatch({ type: "presenceSnapshot", tableId, participants });
          } else if (envelope.kind === "control" && envelope.name === "presence.changed") {
            const payload = envelope.payload !== null && typeof envelope.payload === "object" && !Array.isArray(envelope.payload)
              ? envelope.payload as Record<string, unknown>
              : null;
            const participant = normalizeParticipantPresence(payload?.participant);
            if (participant === null) {
              setConnectionState("degraded");
              dispatch({ type: "issue", issue: issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "websocket" }) });
              return;
            }
            dispatch({ type: "presenceChanged", tableId, participant });
          } else if (envelope.kind === "control" && envelope.name === "table.access_revoked") {
            clearTable();
          } else if (envelope.kind === "control" && envelope.name === "server.draining") {
            plannedDisconnect = true;
            setConnectionState("syncing");
          }
        };
        socket.onclose = (event) => {
          if (rotationTimer !== null) {
            clearTimeout(rotationTimer);
          }
          if (connectionGenerationRef.current !== generation || event.code === 1000) {
            return;
          }
          socketRef.current = null;
          const reconnectSilently = plannedDisconnect || event.code === 1012;
          setConnectionState(reconnectSilently ? "syncing" : navigator.onLine ? "degraded" : "offline");
          dispatch({
            type: "connectionLost",
            ...(reconnectSilently ? {} : { issue: issueFromFailure(new TypeError("socket closed"), "websocket") })
          });
          const delay = Math.min(10_000, 500 * 2 ** attempt) + Math.floor(Math.random() * 300);
          reconnectTimerRef.current = setTimeout(() => void connect(tableId, generation, attempt + 1), delay);
        };
      } catch (error) {
        if (connectionGenerationRef.current !== generation) {
          return;
        }
        setConnectionState(navigator.onLine ? "degraded" : "offline");
        dispatch({ type: "connectionLost", issue: issueForError(error) });
        const delay = Math.min(10_000, 500 * 2 ** attempt) + Math.floor(Math.random() * 300);
        reconnectTimerRef.current = setTimeout(() => void connect(tableId, generation, attempt + 1), delay);
      }
    },
    [authenticatedRequest, clearTable, requestResync]
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
    beginConnectionRef.current = beginConnection;
    return () => {
      beginConnectionRef.current = null;
    };
  }, [beginConnection]);

  useEffect(() => {
    let active = true;
    async function restoreSession() {
      const local = browserStorage("local");
      const session = browserStorage("session");
      const identity = readStoredValue<StoredIdentity>(local, IDENTITY_KEY);
      if (identity === null) {
        setInitializing(false);
        return;
      }
      if (!restoreTable) {
        const access = readStoredValue<StoredAccess>(session, ACCESS_KEY);
        if (access !== null && Date.parse(access.accessExpiresAt) > Date.now() + 30_000) {
          credentialsRef.current = { ...identity, ...access };
        }
        setNickname(identity.nickname);
        setInitializing(false);
        return;
      }
      try {
        const access = readStoredValue<StoredAccess>(session, ACCESS_KEY);
        if (access !== null && Date.parse(access.accessExpiresAt) > Date.now() + 30_000) {
          credentialsRef.current = { ...identity, ...access };
          setNickname(identity.nickname);
        } else {
          await refreshCredentials(identity);
        }
        const storedTable = restoreTable ? readStoredValue<StoredTable>(local, TABLE_KEY) : null;
        if (storedTable !== null && active) {
          const table = normalizeLiveTableProjection(await authenticatedRequest<unknown>(`/v1/tables/${encodeURIComponent(storedTable.tableId)}`));
          if (table !== null) {
            setInviteCode(storedTable.inviteCode ?? null);
            dispatch({ type: "enter", table });
            beginConnection(table.tableId);
          } else {
            removeStoredValue(local, TABLE_KEY);
          }
        }
      } catch (error) {
        if (error instanceof ApiError && (error.code === "SESSION_INVALID" || error.code === "SESSION_INACTIVE")) {
          removeStoredValue(local, IDENTITY_KEY);
          removeStoredValue(session, ACCESS_KEY);
          setNickname(null);
        } else {
          dispatch({ type: "issue", issue: issueForError(error) });
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
  }, [authenticatedRequest, beginConnection, refreshCredentials, restoreTable, stopConnection]);

  useEffect(() => {
    function handleOnline() {
      const tableId = tableStateRef.current.activeTableId;
      if (tableId !== null) {
        beginConnection(tableId);
      }
    }
    function handleOffline() {
      setConnectionState("offline");
      if (tableStateRef.current.activeTableId !== null) {
        dispatch({ type: "connectionLost", issue: issueFromFailure(new TypeError("browser offline"), "websocket") });
      }
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
      dispatch({ type: "issue", issue: null });
      return true;
    } catch (error) {
      dispatch({ type: "issue", issue: issueForError(error) });
      return false;
    } finally {
      setBusy(false);
    }
  }, []);

  const logout = useCallback(async () => {
    setBusy(true);
    clearTable();
    try {
      if (credentialsRef.current !== null) {
        await authenticatedRequest<void>("/v1/guest-sessions/current", { method: "DELETE" });
      }
    } catch {
    } finally {
      credentialsRef.current = null;
      removeStoredValue(browserStorage("local"), IDENTITY_KEY);
      removeStoredValue(browserStorage("session"), ACCESS_KEY);
      setNickname(null);
      setBusy(false);
    }
  }, [authenticatedRequest, clearTable]);

  const createTable = useCallback(async () => {
    setBusy(true);
    try {
      const created = await authenticatedRequest<CreateTableResponse>("/v1/tables", { method: "POST" });
      const table = normalizeLiveTableProjection(created.table);
      if (table === null) {
        throw new ApiError(issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "rest" }), "INVALID_TABLE_PROJECTION");
      }
      clearTable();
      writeStoredValue(browserStorage("local"), TABLE_KEY, { tableId: table.tableId, inviteCode: created.inviteCode });
      setInviteCode(created.inviteCode);
      dispatch({ type: "enter", table });
      beginConnection(table.tableId);
      return table.tableId;
    } catch (error) {
      dispatch({ type: "issue", issue: issueForError(error) });
      return null;
    } finally {
      setBusy(false);
    }
  }, [authenticatedRequest, beginConnection, clearTable]);

  const joinTable = useCallback(
    async (rawInviteCode: string) => {
      const normalizedInviteCode = rawInviteCode.trim().toUpperCase();
      if (!/^[A-Z2-7]{26}$/.test(normalizedInviteCode)) {
        dispatch({
          type: "issue",
          issue: {
            kind: "validation",
            title: "Kode undangan tidak valid",
            detail: "Masukkan 26 karakter yang terdapat pada tautan undangan.",
            retryable: false,
            action: "editInvite",
            source: "browser"
          }
        });
        return null;
      }
      setBusy(true);
      try {
        const table = normalizeLiveTableProjection(await authenticatedRequest<unknown>(`/v1/tables/${encodeURIComponent(normalizedInviteCode)}/join`, { method: "POST" }));
        if (table === null) {
          throw new ApiError(issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "rest" }), "INVALID_TABLE_PROJECTION");
        }
        clearTable();
        writeStoredValue(browserStorage("local"), TABLE_KEY, { tableId: table.tableId, inviteCode: normalizedInviteCode });
        setInviteCode(normalizedInviteCode);
        dispatch({ type: "enter", table });
        beginConnection(table.tableId);
        return table.tableId;
      } catch (error) {
        dispatch({ type: "issue", issue: issueForError(error) });
        return null;
      } finally {
        setBusy(false);
      }
    },
    [authenticatedRequest, beginConnection, clearTable]
  );

  const openTable = useCallback(async (tableId: string) => {
    if (tableStateRef.current.activeTableId === tableId && tableStateRef.current.table !== null) {
      return true;
    }
    setBusy(true);
    try {
      const table = normalizeLiveTableProjection(await authenticatedRequest<unknown>(`/v1/tables/${encodeURIComponent(tableId)}`));
      if (table === null) {
        throw new ApiError(issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "rest" }), "INVALID_TABLE_PROJECTION");
      }
      clearTable();
      writeStoredValue(browserStorage("local"), TABLE_KEY, { tableId });
      dispatch({ type: "enter", table });
      beginConnection(table.tableId);
      return true;
    } catch (error) {
      dispatch({ type: "issue", issue: issueForError(error) });
      return false;
    } finally {
      setBusy(false);
    }
  }, [authenticatedRequest, beginConnection, clearTable]);

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
        await authenticatedRequest<void>(`/v1/tables/${encodeURIComponent(table.tableId)}/leave`, { method: "POST" });
        clearTable();
      } else {
        dispatch({ type: "issue", issue: issueFromServer({ source: "rest" }) });
      }
    } catch (error) {
      dispatch({ type: "issue", issue: issueForError(error) });
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

  const resync = useCallback(() => {
    const tableId = tableStateRef.current.activeTableId;
    if (tableId !== null) {
      requestResync(tableId);
    }
  }, [requestResync]);

  const dismissIssue = useCallback(() => {
    dispatch({ type: "issue", issue: null });
  }, []);

  const dismissNotice = useCallback(() => {
    dispatch({ type: "dismissNotice" });
  }, []);

  const canSendCommand = useCallback((name: CommandName, payload: Record<string, unknown> = {}) => {
    return canSendTableCommand({
      table: projectedTable,
      connected: connectionState === "connected",
      controllerState: tableState.controllerState,
      hasPendingCommand: Object.keys(tableState.pending).length > 0,
    }, name, payload);
  }, [connectionState, projectedTable, tableState.controllerState, tableState.pending]);

  const sendCommand = useCallback((name: CommandName, payload: Record<string, unknown> = {}) => {
    const socket = socketRef.current;
    const table = tableStateRef.current.table;
    if (socket === null || socket.readyState !== WebSocket.OPEN || table === null) {
      dispatch({ type: "issue", issue: issueFromFailure(new TypeError("socket unavailable"), "websocket") });
      return;
    }
    const controllerState = tableStateRef.current.controllerState;
    if ((name === "table.takeover" && controllerState !== "readyToTakeover") || (name !== "table.takeover" && controllerState !== "current")) {
      dispatch({
        type: "issue",
        issue: {
          kind: "conflict",
          title: "Meja masih diselaraskan",
          detail: "Tunggu keadaan terbaru sebelum mengirim aksi berikutnya.",
          retryable: true,
          action: "resync",
          source: "websocket"
        }
      });
      return;
    }
    if (!canSendCommand(name, payload)) {
      return;
    }
    const requestId = createRequestId();
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
    dispatch({ type: "pending", requestId, commandName: name, payload });
    try {
      socket.send(JSON.stringify(command));
    } catch (error) {
      dispatch({ type: "settled", requestId, issue: issueFromFailure(error, "websocket") });
    }
  }, [canSendCommand]);

  useEffect(() => {
    const table = tableState.table;
    if (
      connectionState !== "connected" ||
      tableState.controllerState !== "readyToTakeover" ||
      table?.viewerSeat === undefined ||
      automaticTakeoverRevisionRef.current === table.revision
    ) {
      return;
    }
    automaticTakeoverRevisionRef.current = table.revision;
    sendCommand("table.takeover");
  }, [connectionState, sendCommand, tableState.controllerState, tableState.table]);

  return {
    initializing,
    busy,
    nickname,
    connectionState,
    inviteCode,
    tableState,
    projectedTable,
    createIdentity,
    logout,
    createTable,
    joinTable,
    openTable,
    leaveTable,
    reconnect,
    resync,
    dismissIssue,
    dismissNotice,
    canSendCommand,
    sendCommand
  };
}
