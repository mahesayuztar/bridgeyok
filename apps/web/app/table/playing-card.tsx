import type { CSSProperties } from "react";
import type { Card, Suit } from "../table-state";
import {
  cardKey,
  sortCardsDescending,
  suitLabels,
} from "./gameplay-presentation";

const dummySuitOrder: Suit[] = ["S", "H", "D", "C"];

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
  const className = `physical-card suit-${card.suit.toLowerCase()} card-${variant}`;
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
      onClick={() => onPlay(card)}
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
    >
      <h3>{title}</h3>
      <div className="hand-cards" style={style}>
        {variant === "dummy"
          ? dummySuitOrder.map((suit) => {
              const suitCards = sortCardsDescending(
                cards.filter((card) => card.suit === suit),
              );

              return (
                <div
                  className={`dummy-suit dummy-suit-${suit.toLowerCase()}`}
                  key={suit}
                >
                  <div className="dummy-suit-cards">
                    {suitCards.map((card) => (
                      <PlayingCard
                        card={card}
                        variant="dummy"
                        key={cardKey(card)}
                        disabled={disabled}
                        playable={playableKeys.has(cardKey(card))}
                        {...(onPlay === undefined ? {} : { onPlay })}
                      />
                    ))}
                  </div>
                </div>
              );
            })
          : cards.map((card) => (
              <PlayingCard
                card={card}
                variant={variant}
                key={cardKey(card)}
                disabled={disabled}
                playable={playableKeys.has(cardKey(card))}
                {...(onPlay === undefined ? {} : { onPlay })}
              />
            ))}
      </div>
    </section>
  );
}
