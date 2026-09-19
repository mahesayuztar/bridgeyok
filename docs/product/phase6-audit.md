# Phase 6 — Current-State Audit Evidence

Tanggal: 19 September 2026. Source baseline: `9738c90`.
Tujuan: evidence untuk [implementation plan Phase 6](phase6-persistent-table-ux.md), bukan klaim implementation gate PASS.

## Lingkungan dan cakupan

Audit source mencakup root/web PLAN, route layouts/pages, table/session/reducer/projection/capability, account presence/chat store, gameplay primitives/history/replay/result, globals.css, contract REST dan ADR 0012–0014/0018–0020. Checkpoint GUX/ENG/Phase 5 dibaca sebagai evidence historis, tidak dijalankan ulang seluruhnya.

Browser: Playwright 1.62.1 dengan Chromium headless dari dependency existing. Web Next dev localhost:3106, API Go localhost:8186, disposable PostgreSQL 17 localhost:55436. Migration 00001–00011 berhasil. Tidak menggunakan akun, database atau API produksi. Akun audit terpisah, satu manusia North + tiga bot first-legal existing.

Dua perjalanan browser dijalankan: baseline auction/navigation dan konfirmasi dengan play/dummy. Screenshot dilihat langsung sebagai gambar, selain membaca source dan DOM. Percobaan pertama pass konfirmasi gagal sebelum membuka halaman karena dev server telah berhenti (`ERR_CONNECTION_REFUSED`); server dijalankan ulang dan pass konfirmasi berhasil. Ini bukan temuan bug produk.

Artifacts tersimpan di [phase6-evidence](phase6-evidence/results.json). JSON berisi counters/nama command dan geometry, tanpa ticket/access credential atau raw game payload. Screenshot berisi hanya akun dan deal disposable lokal.

## Reproduction perjalanan yang diamati

1. Jalankan PostgreSQL disposable dan migration existing, API dengan APP_ENV=test serta ALLOWED_ORIGINS web lokal; start Next dev dengan API_BASE_URL dan NEXT_PUBLIC_API_BASE_URL ke API lokal.
2. Chromium 1440×900 → Sign Up → Casual Game → Buat meja. Duduk North, tambah bot E/S/W melalui menu seat, Saya siap, Mulai board.
3. Catat socket open/close dan nama outgoing frame, filter URL `/v1/ws` agar HMR Next tidak masuk hitungan. Catat GET `/v1/tables/{id}`.
4. Capture auction pada 1440×900, 1024×768, 768×1024, 390×844 dan 320×568. Ukur document overflow, keberadaan primary nav dan computed card corner size.
5. Pada pass konfirmasi, pilih 1 lalu NT melalui BiddingBox; bot pass/opening lead. Klik satu legal dummy card, amati ongoing trick. Capture 1440, 390 dan 320 px.
6. Table tidak punya link Friends. Untuk menguji **soft navigation**, probe memanggil router dev runtime `window.next.router.push('/friends')`, bukan `page.goto`/document reload. Ini instrumentation audit, bukan API yang direkomendasikan untuk implementasi. Pada implementasi Phase 6, test harus klik nav asli.
7. Klik link Settings yang nyata; pada 320 px klik History existing dan amati Coming soon. Kembali ke URL table melalui router soft push; tunggu dummy/play surface pulih.
8. Simpan metadata network redacted dan screenshots, tutup browser. Tidak mengirim explicit leave pada perjalanan ini.

Random deal bukan fixture reproduksi rank tertentu; temuan navigation dan layout yang dilaporkan tidak membutuhkan deal persis sama. Gate Phase 6 nantinya wajib memakai fixture deterministic untuk dense dummy/all-seat geometry.

## Hasil runtime yang dikonfirmasi

### R1 — Navigasi workspace mengubah lifecycle realtime

Pada pass konfirmasi, table socket diberi ID audit 3 (ID 1–2 berasal dari account/bootstrap sebelumnya). Setelah marker `navigationBaseline`:

| Aksi | Network yang teramati |
|---|---|
| Table → Friends | Socket table 3 close, socket account 4 open |
| Friends → Settings | Tidak ada penggantian app socket yang tercatat pada segmen ini |
| Klik History | Toast `Coming soon`; tidak ada workspace History |
| Settings → Table | Socket account 4 close, GET table, socket 5 open, `table.resume`, `table.takeover` |

Delta keseluruhan setelah marker baseline: **2 open, 2 close, 1 GET table, 1 resume, 1 takeover**. Tidak ada `leave` yang dikirim. Ini membuktikan perbedaan antara mempertahankan membership server dan mempertahankan live client session.

Resume pada return menunjukkan sebagian client state bisa dipertahankan oleh runtime, tetapi effect/socket lifecycle tetap restart. Jangan menyimpulkan semua state selalu hilang; invariant Phase 6 yang gagal adalah connection/refetch/ownership continuity. `useTableSession` cleanup/restore di source menjelaskan hasil tersebut.

### R2 — Table tidak memiliki navigation aplikasi

Pada seluruh lima ukuran auction, query `nav[aria-label="Navigasi utama"]` tidak ditemukan. Table hanya mempunyai navbar gameplay. Friends/Settings memakai account sidebar/bottom navigation terpisah. Ini bukan sekadar masalah label atau menambah tombol Friends.

### R3 — Readability dan targets perlu dibedakan dari overflow

| Auction viewport | Horizontal document overflow | Computed `.card-corner` font | Buttons dengan salah satu dimensi <44 px |
|---|---:|---:|---:|
| 1440×900 | Tidak | 24.48 px | 16 |
| 1024×768 | Tidak | 19.2 px | 16 |
| 768×1024 | Tidak | 17.92 px | 16 |
| 390×844 | Tidak | 17.92 px | 16 |
| 320×568 | Tidak | 17.92 px | 16 |

Count ini screening DOM bounding boxes dengan width >0, mencakup disabled controls; bukan audit hit-testing/accessibility lengkap. Tidak mengklaim 16 kegagalan WCAG atau 16 kontrol playable. Pada play desktop, probe juga mencatat participant bars sekitar 382×18 px / 18×592 px dan Undo disabled 72×35 px. Kandidat Invite 176×43 px perlu dicek visibility/top-layer sebelum dianggap target aktif.

Visual 320 px menunjukkan navbar mengambil dua baris, nama North terpotong, hand/dummy rapat, dan spacing antara top-trick card dan dummy sangat dekat/beririsan pada frame tangkapan. Karena motion masih berjalan, capture itu adalah kandidat pemeriksaan collision lintas animation stage, bukan bukti semua steady-state layout gagal. Tidak ada klaim geometry seluruh orientasi Dummy telah diuji ulang.

### R4 — Placeholder dan copy masih nyata

History di account nav mengeluarkan Coming soon. Friends menampilkan eyebrow “Main bersama”, heading Friends, penjelasan mutual follow, dan penjelasan serupa lagi pada empty state. Settings memiliki eyebrow Settings dengan heading Profile. Mobile bottom nav lima tujuan memakai label kecil dan dua tujuan belum berfungsi. Phase 6 sebaiknya mengurangi pengulangan tanpa menghapus informasi mutual-follow pada konteks yang membutuhkannya.

## Screenshot yang ditinjau

- [Auction desktop 1440×900](phase6-evidence/auction-1440.png)
- [Auction mobile 320×568](phase6-evidence/auction-320.png)
- [Play/dummy/current trick desktop 1440×900](phase6-evidence/play-1440.png)
- [Play/dummy/current trick mobile 320×568](phase6-evidence/play-320.png)
- [Friends desktop](phase6-evidence/friends-desktop.png)
- [Settings mobile](phase6-evidence/settings-mobile.png)

## Temuan source penting yang belum diuji sebagai runtime gate baru

- `useTableSession` dipanggil oleh BridgeTable dan LobbyClient; cleanup menutup socket, restore GET table; `AccountPresence` membuka account socket sendiri ketika tidak compact.
- `chatStore.attach` menyimpan satu socket; dua owner berpotensi saling mengganti sumber jika shell digabung tanpa refactor ownership.
- `BoardResult` memiliki timeout lima detik dan document click yang dapat memanggil next board. Ini risiko langsung ketika table menjadi persistent; source-confirmed, tidak dieksekusi dalam audit selesai-board kali ini.
- Auction shortcut memakai listener window yang perlu visibility/focus guard ketika Friends/Settings berada di depan.
- `AuctionTable` scroll-to-bottom setiap calls.length berubah; `TrickIndicator` reset selection pada trigger dan belum merender leader/winner meski type menyediakan keduanya.
- ScoreSheet/board replay sudah ada; Team Match final IMP sudah ada. Scope awal plan yang menyebut Friends/chat excluded telah disupersede ADR.
- Match replay entitlement sebelumnya disegarkan saat table re-entry; persistent session memerlukan invalidation authoritative terarah.
- GameProjection tidak menyediakan game clock pemain. Tidak perlu membangun clock demi menyamakan wording brief.

## Batas evidence

Audit ini **tidak** menjalankan full four/eight-human regression, finished-board lifecycle, reconnect/offline, rotation 285 detik, Back/Forward, browser zoom, screen reader, contrast meter, real touch drag, history panjang atau seluruh orientasi seat. Semua masuk validation plan, bukan diklaim PASS. Mouse clicks form/nav/bid/card sudah digunakan; viewport mobile resize bukan emulasi bukti real touch.

Tidak ada source aplikasi/engine yang sengaja diubah. Next dev menghasilkan perubahan `next-env.d.ts`; perubahan generated tersebut dikembalikan ke baseline setelah audit. API/dev server dan container PostgreSQL audit dihentikan setelah pengambilan evidence.
