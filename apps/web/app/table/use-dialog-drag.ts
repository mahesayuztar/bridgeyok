import { useEffect, useRef, type KeyboardEvent, type PointerEvent } from "react";

export function useDialogDrag() {
  const panelRef = useRef<HTMLElement | null>(null);
  const gestureRef = useRef<{ pointerId: number; x: number; y: number; left: number; top: number } | null>(null);

  function movePanel(panel: HTMLElement, left: number, top: number) {
    const viewport = window.visualViewport;
    const width = viewport?.width ?? window.innerWidth;
    const height = viewport?.height ?? window.innerHeight;
    const offsetLeft = viewport?.offsetLeft ?? 0;
    const offsetTop = viewport?.offsetTop ?? 0;
    const rect = panel.getBoundingClientRect();
    Object.assign(panel.style, {
      position: "fixed",
      margin: "0",
      inset: "auto",
      left: `${Math.max(offsetLeft + 4, Math.min(left, offsetLeft + width - rect.width - 4))}px`,
      top: `${Math.max(offsetTop + 4, Math.min(top, offsetTop + height - rect.height - 4))}px`,
    });
    panelRef.current = panel;
  }

  useEffect(() => {
    function keepVisible() {
      const panel = panelRef.current;
      if (!panel?.isConnected || panel.getClientRects().length === 0) return;
      const rect = panel.getBoundingClientRect();
      movePanel(panel, rect.left, rect.top);
    }
    window.addEventListener("resize", keepVisible);
    window.visualViewport?.addEventListener("resize", keepVisible);
    window.visualViewport?.addEventListener("scroll", keepVisible);
    return () => {
      window.removeEventListener("resize", keepVisible);
      window.visualViewport?.removeEventListener("resize", keepVisible);
      window.visualViewport?.removeEventListener("scroll", keepVisible);
    };
  }, []);

  function finishDrag(event: PointerEvent<HTMLElement>) {
    if (gestureRef.current?.pointerId !== event.pointerId) return;
    gestureRef.current = null;
    event.currentTarget.removeAttribute("data-dragging");
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  }

  return {
    "data-dialog-drag-handle": true,
    tabIndex: 0,
    "aria-description": "Geser untuk memindahkan. Gunakan tombol panah saat judul difokuskan.",
    onPointerDown(event: PointerEvent<HTMLElement>) {
      if (!event.isPrimary || event.button !== 0 || gestureRef.current !== null ||
        (event.target instanceof Element && event.target.closest("button, a, input, select, textarea"))) return;
      const panel = event.currentTarget.closest<HTMLElement>("dialog, [role='dialog']");
      if (!panel) return;
      const rect = panel.getBoundingClientRect();
      gestureRef.current = { pointerId: event.pointerId, x: event.clientX, y: event.clientY, left: rect.left, top: rect.top };
      panelRef.current = panel;
      event.currentTarget.setPointerCapture(event.pointerId);
      event.currentTarget.setAttribute("data-dragging", "true");
      event.preventDefault();
    },
    onPointerMove(event: PointerEvent<HTMLElement>) {
      const gesture = gestureRef.current;
      const panel = panelRef.current;
      if (!gesture || !panel || gesture.pointerId !== event.pointerId) return;
      movePanel(panel, gesture.left + event.clientX - gesture.x, gesture.top + event.clientY - gesture.y);
    },
    onPointerUp: finishDrag,
    onPointerCancel: finishDrag,
    onLostPointerCapture: finishDrag,
    onKeyDown(event: KeyboardEvent<HTMLElement>) {
      if (event.target !== event.currentTarget || !["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(event.key)) return;
      const panel = event.currentTarget.closest<HTMLElement>("dialog, [role='dialog']");
      if (!panel) return;
      event.preventDefault();
      event.stopPropagation();
      const rect = panel.getBoundingClientRect();
      const distance = event.shiftKey ? 40 : 10;
      movePanel(panel, rect.left + (event.key === "ArrowLeft" ? -distance : event.key === "ArrowRight" ? distance : 0), rect.top + (event.key === "ArrowUp" ? -distance : event.key === "ArrowDown" ? distance : 0));
    },
  };
}
