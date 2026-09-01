import type { CSSProperties } from "react";
import type { Card } from "../table-state";
import { cardKey, suitLabels } from "./gameplay-presentation";
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
  const className = `physical-card suit-${card.suit.toLowerCase()} card-${variant}${drag.dragging ? " is-dragging" : ""}`;
  const label = `${rank} ${suitLabels[card.suit]}`;

  if (onPlay === undefined) {
    return (
      <span className={className} aria-label={label}>
        {content}
      </span>
    );
  }
  return (
    <button
      className={className}
      type="button"
      disabled={disabled || !playable}
      onClick={() => {
        if (!drag.shouldSuppressClick()) onPlay(card);
      }}
      onPointerDown={drag.handlePointerDown}
      onPointerMove={drag.handlePointerMove}
      onPointerUp={drag.handlePointerUp}
      onPointerCancel={drag.handlePointerCancel}
      onLostPointerCapture={drag.handleLostPointerCapture}
      style={{
        "--drag-x": `${drag.offset.x}px`,
        "--drag-y": `${drag.offset.y}px`,
      } as CSSProperties}
      data-dragging={drag.dragging}
      aria-label={`Mainkan ${label}`}
    >
      {content}
    </button>
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
}: {
  cards: Card[];
  title: string;
  variant?: "hand" | "dummy";
  playableCards?: Card[];
  disabled?: boolean;
  onPlay?: (card: Card) => void;
  className?: string;
}) {
  const playableKeys = new Set(playableCards.map(cardKey));
  const style = { "--card-count": Math.max(cards.length, 1) } as CSSProperties;
  return (
    <section
      className={`bridge-hand ${className}`}
      data-variant={variant}
      aria-label={title}
      style={style}
    >
      <div className="hand-cards">
        {cards.map((card, _cardIndex) => (
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
    </section>
  );
}
