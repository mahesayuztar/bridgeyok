import { useEffect, useMemo, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";

export type PositionAnalysis = components["schemas"]["PositionAnalysis"];
export type LoadPositionAnalysis = (
  boardId: string,
  positionKey: string,
  step: number | undefined,
  signal: AbortSignal,
) => Promise<PositionAnalysis>;

export function usePositionAnalysis(
  load: LoadPositionAnalysis,
  boardId: string | undefined,
  positionKey: string,
  enabled: boolean,
  step?: number,
) {
  const request = useMemo(
    () => ({ load, boardId, positionKey, enabled, step }),
    [load, boardId, positionKey, enabled, step],
  );
  const [state, setState] = useState<{
    request: typeof request;
    result?: PositionAnalysis;
    failed?: boolean;
  } | null>(null);

  useEffect(() => {
    if (!request.enabled || !request.boardId) return;
    const controller = new AbortController();
    request.load(request.boardId, request.positionKey, request.step, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        if (result.boardId !== request.boardId || result.positionKey !== request.positionKey)
          throw new Error("DDS position mismatch");
        setState({ request, result });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ request, failed: true });
      });
    return () => controller.abort();
  }, [request]);

  const current = enabled && state?.request === request ? state : null;
  return {
    result: current?.result,
    failed: current?.failed === true,
    pending: enabled && current === null,
  };
}
