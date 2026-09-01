"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  gameplayMotionEvents,
  gameplayMotionKey,
  type GameplayMotionEvent,
} from "../gameplay-motion";
import type { GameProjection, Seat, Trick } from "../table-state";

export type GameplayMotionFrame = {
  trick: Trick;
  stage: "idle" | "moving" | "winner" | "collecting";
  movingSeat?: Seat;
};

const emptyTrick: Trick = { plays: [] };

export function useGameplayMotion(game: GameProjection | undefined) {
  const [frame, setFrame] = useState<GameplayMotionFrame>({
    trick: game?.currentTrick ?? emptyTrick,
    stage: "idle",
  });
  const [isAnimating, setIsAnimating] = useState(false);
  const previousGameRef = useRef(game);
  const previousKeyRef = useRef(game === undefined ? null : gameplayMotionKey(game));
  const queueRef = useRef<GameplayMotionEvent[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const finishRef = useRef<(() => void) | null>(null);
  const reducedMotionRef = useRef(false);
  const runNextRef = useRef<() => void>(() => undefined);

  const runNext = useCallback(() => {
    if (timerRef.current !== null) return;
    const event = queueRef.current.shift();
    if (event === undefined) {
      setIsAnimating(false);
      return;
    }
    if (event.kind === "sync") {
      setFrame({ trick: event.trick, stage: "idle" });
      queueMicrotask(() => runNextRef.current());
      return;
    }

    setIsAnimating(true);
    setFrame({
      trick: event.trick,
      stage:
        event.kind === "play"
          ? "moving"
          : event.kind === "winner"
            ? "winner"
            : "collecting",
      ...(event.kind === "play" ? { movingSeat: event.movingSeat } : {}),
    });
    const duration = reducedMotionRef.current
      ? event.kind === "winner"
        ? 120
        : 40
      : event.kind === "play"
        ? 200
        : event.kind === "winner"
          ? 650
          : 180;
    const finish = () => {
      timerRef.current = null;
      finishRef.current = null;
      if (event.kind === "collect") {
        setFrame({ trick: emptyTrick, stage: "idle" });
      } else if (event.kind === "play") {
        setFrame({ trick: event.trick, stage: "idle" });
      }
      runNextRef.current();
    };
    finishRef.current = finish;
    timerRef.current = setTimeout(finish, duration);
  }, []);

  useEffect(() => {
    runNextRef.current = runNext;
  }, [runNext]);

  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    reducedMotionRef.current = media.matches;
    function updateReducedMotion(event: MediaQueryListEvent) {
      reducedMotionRef.current = event.matches;
    }
    media.addEventListener("change", updateReducedMotion);
    return () => media.removeEventListener("change", updateReducedMotion);
  }, []);

  useEffect(() => {
    if (game === undefined) {
      previousGameRef.current = undefined;
      previousKeyRef.current = null;
      queueRef.current = [];
      queueRef.current.push({ kind: "sync", trick: emptyTrick });
      queueMicrotask(() => runNextRef.current());
      return;
    }
    const currentKey = gameplayMotionKey(game);
    if (previousKeyRef.current === currentKey) return;
    const previousGame = previousGameRef.current;
    previousGameRef.current = game;
    previousKeyRef.current = currentKey;
    if (previousGame === undefined) {
      queueRef.current.push({ kind: "sync", trick: game.currentTrick });
      queueMicrotask(() => runNextRef.current());
      return;
    }
    queueRef.current.push(...gameplayMotionEvents(previousGame, game));
    queueMicrotask(() => runNextRef.current());
  }, [game]);

  useEffect(
    () => () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current);
      timerRef.current = null;
      finishRef.current = null;
      queueRef.current = [];
    },
    [],
  );

  function skipCurrent() {
    if (timerRef.current === null || finishRef.current === null) return;
    clearTimeout(timerRef.current);
    finishRef.current();
  }

  return { frame, isAnimating, skipCurrent };
}
