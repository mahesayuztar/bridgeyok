"use client";

import { useEffect, useRef, useState } from "react";
import type { LiveTableProjection } from "./table-state";
import {
  shouldPlayTurnCue,
  TURN_CUE_DURATION_SECONDS,
  TURN_CUE_PEAK_GAIN,
  turnCueState,
} from "./turn-cue";

const TURN_AUDIO_MUTED_KEY = "bridgeyok.turnAudioMuted";

export function useTurnAudio(table: LiveTableProjection | null) {
  const [muted, setMutedState] = useState(() => {
    if (typeof window === "undefined") return false;
    try {
      return window.localStorage.getItem(TURN_AUDIO_MUTED_KEY) === "true";
    } catch {
      return false;
    }
  });
  const interactedRef = useRef(false);
  const previousStateRef = useRef<ReturnType<typeof turnCueState>>(null);

  useEffect(() => {
    function markInteraction() {
      interactedRef.current = true;
    }
    window.addEventListener("pointerdown", markInteraction, { once: true });
    window.addEventListener("keydown", markInteraction, { once: true });
    return () => {
      window.removeEventListener("pointerdown", markInteraction);
      window.removeEventListener("keydown", markInteraction);
    };
  }, []);

  useEffect(() => {
    const currentState = table === null ? null : turnCueState(table);
    const shouldPlay = shouldPlayTurnCue(previousStateRef.current, currentState);
    previousStateRef.current = currentState;
    if (!shouldPlay || muted || !interactedRef.current) return;

    try {
      const AudioContextClass = window.AudioContext;
      const context = new AudioContextClass();
      if (context.state === "suspended") {
        void context.resume().catch(() => undefined);
      }
      const oscillator = context.createOscillator();
      const gain = context.createGain();
      const now = context.currentTime;
      oscillator.type = "sine";
      oscillator.frequency.setValueAtTime(660, now);
      oscillator.frequency.exponentialRampToValueAtTime(
        520,
        now + TURN_CUE_DURATION_SECONDS - 0.04,
      );
      gain.gain.setValueAtTime(0.0001, now);
      gain.gain.exponentialRampToValueAtTime(
        TURN_CUE_PEAK_GAIN,
        now + 0.018,
      );
      gain.gain.exponentialRampToValueAtTime(
        0.0001,
        now + TURN_CUE_DURATION_SECONDS,
      );
      oscillator.connect(gain);
      gain.connect(context.destination);
      oscillator.start(now);
      oscillator.stop(now + TURN_CUE_DURATION_SECONDS + 0.01);
      oscillator.addEventListener("ended", () => void context.close(), {
        once: true,
      });
    } catch {
      return;
    }
  }, [muted, table]);

  function setMuted(nextMuted: boolean) {
    setMutedState(nextMuted);
    try {
      window.localStorage.setItem(TURN_AUDIO_MUTED_KEY, String(nextMuted));
    } catch {
      return;
    }
  }

  return { muted, setMuted };
}
