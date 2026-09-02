import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import type { Card, Contract, Suit, VisualPosition } from "../table-state";
import {
  cardKey,
  organizeCardsForContract,
  suitOrderForContract,
  suitLabels,
} from "./gameplay-presentation";
import { useCardDrag } from "./use-card-drag";

export function PlayingCard({
  card,
  variant,
  disabled = false,
  playable = false,
  onPlay,
}: {
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
}: {
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
  const organizedCards = organizeCardsForContract(cards, contractStrain);
  const sideDummy =
    variant === "dummy" && (position === "left" || position === "right");
  const cardGroups: Array<{
    key: string;
    suit?: Suit;
    cards: Card[];
  }> = sideDummy
    ? suitOrderForContract(contractStrain).map((suit) => ({
        key: suit,
        suit,
        cards: organizedCards.filter((card) => card.suit === suit),
      }))
    : [{ key: "hand", cards: organizedCards }];
  const style = { "--card-count": Math.max(cards.length, 1) } as CSSProperties;
  return (
    <section
      className={`bridge-hand ${className}`}
      data-variant={variant}
      data-layout={sideDummy ? "suit-groups" : "fan"}
      aria-label={title}
      style={style}
    >
      <div className="hand-cards">
        {cardGroups.map((cardGroup) => (
          <div
            className={`hand-card-group${cardGroup.suit === undefined ? "" : " dummy-suit-group"}`}
            key={cardGroup.key}
            {...(cardGroup.suit === undefined
              ? {}
              : {
                  "data-suit": cardGroup.suit,
                  "aria-label": `${suitLabels[cardGroup.suit]} ${cardGroup.cards.length} kartu`,
                })}
          >
            {cardGroup.cards.length === 0 && cardGroup.suit !== undefined ? (
              <span
                className={`dummy-void physical-card suit-${cardGroup.suit.toLowerCase()}`}
                aria-hidden="true"
              >
                {suitLabels[cardGroup.suit]}
              </span>
            ) : null}
            {cardGroup.cards.map((card, _cardIndex) => (
              <span
                className="hand-card-slot"
                data-card-index={_cardIndex}
                key={cardKey(card)}
              >
                <PlayingCard
                  card={card}
                  variant={variant}
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
