"use client";
import { PlayingCard } from "./table/playing-card";

export function WelcomeCards() {
  return <div className="welcome-cards" aria-label="Empat kartu As">{(["S", "H", "D", "C"] as const).map(suit => <PlayingCard key={suit} card={{ suit, rank: "A" }} variant="hand" />)}</div>;
}
