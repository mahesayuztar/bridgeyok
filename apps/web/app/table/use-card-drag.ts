"use client";

import { useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

const DRAG_THRESHOLD = 6;

export function useCardDrag(onDrop: () => void) {
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [origin, setOrigin] = useState<{
    left: number;
    top: number;
    width: number;
    height: number;
  } | null>(null);
  const draggingRef = useRef(false);
  const pointerRef = useRef<{
    id: number;
    startX: number;
    startY: number;
    completed: boolean;
  } | null>(null);
  const suppressClickRef = useRef(false);

  function resetDrag() {
    pointerRef.current = null;
    draggingRef.current = false;
    setDragging(false);
    setOffset({ x: 0, y: 0 });
    setOrigin(null);
  }

  function handlePointerDown(event: ReactPointerEvent<HTMLButtonElement>) {
    if (pointerRef.current !== null || !event.isPrimary || event.button !== 0) return;
    pointerRef.current = {
      id: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      completed: false,
    };
    const bounds = event.currentTarget.getBoundingClientRect();
    setOrigin({
      left: bounds.left,
      top: bounds.top,
      width: bounds.width,
      height: bounds.height,
    });
    event.currentTarget.setPointerCapture(event.pointerId);
    setDragging(true);
  }

  function handlePointerMove(event: ReactPointerEvent<HTMLButtonElement>) {
    const pointer = pointerRef.current;
    if (pointer === null || pointer.id !== event.pointerId || pointer.completed) return;
    const x = event.clientX - pointer.startX;
    const y = event.clientY - pointer.startY;
    if (!draggingRef.current && Math.hypot(x, y) < DRAG_THRESHOLD) return;
    event.preventDefault();
    draggingRef.current = true;
    setOffset({ x, y });
  }

  function handlePointerUp(event: ReactPointerEvent<HTMLButtonElement>) {
    const pointer = pointerRef.current;
    if (pointer === null || pointer.id !== event.pointerId || pointer.completed) return;
    pointer.completed = true;
    if (draggingRef.current) {
      suppressClickRef.current = true;
      const board = document.querySelector<HTMLElement>(".board-play-zone");
      const bounds = board?.getBoundingClientRect();
      if (
        bounds !== undefined &&
        event.clientX >= bounds.left &&
        event.clientX <= bounds.right &&
        event.clientY >= bounds.top &&
        event.clientY <= bounds.bottom
      ) {
        onDrop();
      }
    }
    resetDrag();
  }

  function handlePointerCancel(event: ReactPointerEvent<HTMLButtonElement>) {
    if (pointerRef.current?.id !== event.pointerId) return;
    suppressClickRef.current = draggingRef.current;
    resetDrag();
  }

  function handleLostPointerCapture(event: ReactPointerEvent<HTMLButtonElement>) {
    if (pointerRef.current?.id !== event.pointerId) return;
    suppressClickRef.current = draggingRef.current;
    resetDrag();
  }

  function shouldSuppressClick() {
    if (!suppressClickRef.current) return false;
    suppressClickRef.current = false;
    return true;
  }

  return {
    dragging,
    offset,
    origin,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
    handlePointerCancel,
    handleLostPointerCapture,
    shouldSuppressClick,
  };
}
