import type { components } from "@bridgeyok/contracts/openapi";
import Link from "next/link";

const API_BASE_URL = process.env.API_BASE_URL ?? "http://localhost:8080";

type HealthResponse = components["schemas"]["HealthResponse"];

type ApiAvailability = {
  state: "ready" | "starting" | "offline";
  label: string;
  detail: string;
};

export const dynamic = "force-dynamic";

async function getApiAvailability(): Promise<ApiAvailability> {
  try {
    const response = await fetch(new URL("/health/ready", API_BASE_URL), {
      cache: "no-store",
      signal: AbortSignal.timeout(2500)
    });
    const health = (await response.json()) as HealthResponse;

    if (response.ok && health.status === "ready") {
      return {
        state: "ready",
        label: "Fondasi siap",
        detail: "Web, API, dan PostgreSQL merespons normal."
      };
    }
    return {
      state: "starting",
      label: "Sedang dibangunkan",
      detail: "Layanan gratis dapat memerlukan beberapa saat setelah lama tidak digunakan."
    };
  } catch {
    return {
      state: "offline",
      label: "API belum terhubung",
      detail: "Halaman web tetap tersedia sementara layanan belakang disiapkan."
    };
  }
}

export default async function HomePage() {
  const availability = await getApiAvailability();

  return (
    <div className="site-shell">
      <header className="site-header">
        <Link className="wordmark" href="/" aria-label="BridgeYok, halaman utama">
          BridgeYok
        </Link>
        <span className="phase-label">Foundation / 01</span>
      </header>

      <main>
        <section className="hero" aria-labelledby="hero-title">
          <div className="hero-copy">
            <p className="eyebrow">Bridge online untuk meja sendiri</p>
            <h1 id="hero-title">Main bareng. Tetap dekat, walau beda tempat.</h1>
            <p className="hero-summary">
              BridgeYok sedang dibangun sebagai ruang bermain yang ringan, jelas, dan tahan saat koneksi tidak sempurna.
              Selalu gratis, tanpa pembayaran.
            </p>
            <a className="text-link" href="#status">
              Lihat kesiapan fondasi <span aria-hidden="true">↓</span>
            </a>
          </div>
          <p className="hero-note">
            Empat kursi.
            <br />
            Satu meja.
            <br />
            Banyak cerita.
          </p>
        </section>

        <section className="foundation" id="status" aria-labelledby="status-title">
          <div>
            <p className="eyebrow">Status Phase 1</p>
            <h2 id="status-title">Jalur dasar sudah tersambung.</h2>
          </div>
          <div className="runtime-status" data-state={availability.state} role="status" aria-live="polite">
            <span className="status-mark" aria-hidden="true" />
            <div>
              <strong>{availability.label}</strong>
              <p>{availability.detail}</p>
            </div>
          </div>
        </section>
      </main>

      <footer className="site-footer">
        <span>BridgeYok</span>
        <span>Gratis untuk semua meja.</span>
      </footer>
    </div>
  );
}
