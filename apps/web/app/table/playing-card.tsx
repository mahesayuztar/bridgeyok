import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import type { Card, Contract, VisualPosition } from "../table-state";
import {
  cardKey,
  groupCardsForContract,
  organizeCardsForContract,
  suitLabels,
} from "./gameplay-presentation";
import { useCardDrag } from "./use-card-drag";

export function PlayingCard({
  card,
  variant,
  disabled = false,
  playable = false,
  onPlay,
  prediction,
}: {
  prediction?: number | "pending" | undefined;
  card: Card;
  variant: "hand" | "dummy" | "trick";
  disabled?: boolean;
  playable?: boolean;
  onPlay?: (card: Card) => void;
}) {
  const drag = useCardDrag(() => onPlay?.(card));
  const canPlay = onPlay !== undefined && playable && !disabled;
  const rank = card.rank === "T" ? "10" : card.rank;
  const content = (
    <>
      {prediction === undefined ? null : (
        <span className="card-prediction" aria-label={prediction === "pending" ? "Menghitung trick" : `${prediction} predicted tricks`}>
          {prediction === "pending" ? <span className="dds-spinner" role="status" /> : prediction}
        </span>
      )}
      <span className="card-corner">
        <strong>{rank}</strong>
        <span>{suitLabels[card.suit]}</span>
      </span>
      <span className="card-suit" aria-hidden="true">
        {suitLabels[card.suit]}
      </span>
    </>
  );
  const baseClassName = `physical-card suit-${card.suit.toLowerCase()} card-${variant}`;
  const className = `${baseClassName}${drag.dragging ? " is-dragging" : ""}`;
  const label = `${rank} ${suitLabels[card.suit]}`;

  if (onPlay === undefined) {
    return (
      <span className={className} aria-label={label}>
        {content}
      </span>
    );
  }
  return (
    <>
      <button
        className={className}
        type="button"
        disabled={!canPlay}
        onClick={() => {
          if (!drag.shouldSuppressClick()) onPlay(card);
        }}
        {...(canPlay
          ? {
              onPointerDown: drag.handlePointerDown,
              onPointerMove: drag.handlePointerMove,
              onPointerUp: drag.handlePointerUp,
              onPointerCancel: drag.handlePointerCancel,
              onLostPointerCapture: drag.handleLostPointerCapture,
            }
          : {})}
        data-dragging={drag.dragging}
        aria-label={`Mainkan ${label}`}
      >
        {content}
      </button>
      {drag.dragging && drag.origin !== null && typeof document !== "undefined"
        ? createPortal(
            <span
              className={`${baseClassName} card-drag-preview`}
              style={{
                top: drag.origin.top,
                left: drag.origin.left,
                width: drag.origin.width,
                height: drag.origin.height,
                transform: `translate3d(${drag.offset.x}px, ${drag.offset.y}px, 0) rotate(1deg)`,
              }}
              aria-hidden="true"
            >
              {content}
            </span>,
            document.body,
          )
        : null}
    </>
  );
}

export function BridgeHand({
  cards,
  title,
  variant = "hand",
  playableCards = [],
  disabled = false,
  onPlay,
  className = "",
  contractStrain,
  position,
  predictions,
  analysisPending = false,
  analysisCards,
}: {
  predictions?: Array<{ card: Card; tricks: number }> | undefined;
  analysisPending?: boolean;
  analysisCards?: Card[] | undefined;
  cards: Card[];
  title: string;
  variant?: "hand" | "dummy";
  playableCards?: Card[];
  disabled?: boolean;
  onPlay?: (card: Card) => void;
  className?: string;
  contractStrain: Contract["strain"] | undefined;
  position?: VisualPosition;
}) {
  const playableKeys = new Set(playableCards.map(cardKey));
  const analysisKeys = new Set((analysisCards ?? playableCards).map(cardKey));
  const organizedCards = organizeCardsForContract(cards, contractStrain);
  const sideDummy =
    variant === "dummy" && (position === "left" || position === "right");
  const cardGroups = sideDummy
    ? groupCardsForContract(cards, contractStrain)
    : [{ key: "hand", suit: undefined, suitIndex: 0, cards: organizedCards }];
  const style = {
    "--card-count": Math.max(cards.length, 1),
    ...(sideDummy
      ? { "--side-dummy-suit-count": Math.max(cardGroups.length, 1) }
      : {}),
  } as CSSProperties;
  return (
    <section
      className={`bridge-hand ${className}`}
      data-variant={variant}
      data-layout={sideDummy ? "suit-groups" : "fan"}
      aria-label={title}
      style={style}
    >
      <div className="hand-cards">
        {cardGroups.map((cardGroup, _cardGroupIndex) => (
          <div
            className={`hand-card-group${cardGroup.suit === undefined ? "" : " dummy-suit-group"}`}
            key={cardGroup.key}
            style={
              sideDummy
                ? ({
                    "--side-dummy-suit-row": _cardGroupIndex + 1,
                  } as CSSProperties)
                : undefined
            }
            {...(cardGroup.suit === undefined
              ? {}
              : {
                  "data-suit": cardGroup.suit,
                  "data-density":
                    cardGroup.cards.length >= 8 ? "tight" : "normal",
                  "aria-label": `${suitLabels[cardGroup.suit]} ${cardGroup.cards.length} kartu`,
                })}
          >
            {cardGroup.cards.map((card, _cardIndex) => (
              <span
                className="hand-card-slot"
                data-card-index={_cardIndex}
                key={cardKey(card)}
              >
                <PlayingCard
                  card={card}
                  variant={variant}
                  prediction={analysisKeys.has(cardKey(card)) ? predictions?.find((entry) => cardKey(entry.card) === cardKey(card))?.tricks ?? (analysisPending ? "pending" : undefined) : undefined}
                  disabled={disabled}
                  playable={playableKeys.has(cardKey(card))}
                  {...(onPlay === undefined ? {} : { onPlay })}
                />
              </span>
            ))}
          </div>
        ))}
      </div>
    </section>
  );
}
