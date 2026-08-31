# BridgeYok Web Roadmap

> Status: **PLANNED — GUX Gameplay UX Reliability & Interaction Refactor belum dimulai; Work 1–6 lama complete dan browser/E2E baseline masih pending**
> Last updated: 1 September 2026
> Scope: guest entry, lobby navigation, actionable errors, controller takeover, table UX/layout, dan objective GUX frontend-first yang regression-sensitive.
> Implementation must not begin until the visual-reference and projection audits in Work 0 are complete.

Dokumen ini adalah roadmap khusus `apps/web`. `AGENTS.md` tetap menjadi aturan permanen repository dan root `PLAN.md` tetap menjadi roadmap produk lintas komponen. Bila ada konflik, urutan keputusan tetap: permintaan eksplisit pengguna → `AGENTS.md` → root `PLAN.md` → dokumen ini → arsitektur yang sudah ada.

## 1. Outcome utama

Web harus mempunyai tiga konteks yang jelas:

1. Landing page untuk memperkenalkan produk dan membuat guest identity.
2. Lobby page terpisah setelah guest berhasil masuk.
3. Dedicated bridge client ketika pengguna berada di meja.

Prioritas tertinggi refactor ini adalah:

- setelah **Masuk sebagai tamu**, navigasi berpindah ke halaman baru `/lobby`;
- `/lobby` tidak memiliki hero marketing: hanya navbar ringkas dan application content;
- heading terbesar dan pertama di `/lobby` adalah sapaan personal, misalnya `Halo, Mahesa`;
- setiap kegagalan join/session/network mempunyai pesan, penyebab, dan tindakan lanjut yang spesifik;
- **Ambil alih kendali** benar-benar memulihkan controller yang stale dan memberikan konfirmasi yang terlihat;
- AUCTION, PLAY, dan SCORE memakai satu table shell persisten yang secara struktur, information hierarchy, spatial layout, dan interaction model sekitar 90% familiar bagi pemain Bridge Base Online (BBO);
- 10% diferensiasi produk dipakai untuk warna, tipografi, responsivitas, aksesibilitas, feedback koneksi, dan kualitas interaksi—bukan untuk mengganti struktur meja BBO.

Ini bukan pekerjaan “mempercantik meja”. Gameplay viewport harus terasa sebagai aplikasi bridge khusus, bukan landing page, dashboard, atau kumpulan card/panel.

## 2. Batas pekerjaan

### Harus dipertahankan

- Pure bridge engine dan seluruh aturan auction/play/scoring.
- Server-authoritative validation untuk call, card, turn, dummy, revision, dan controller epoch.
- WebSocket envelope, ack/error/event ordering, reconnect/resume, snapshot fallback, dan no automatic mutation retry.
- Recipient projection dan perlindungan hidden hand.
- Durable persistence, idempotency, actor/table state machine, dan room lifecycle.
- Existing design tokens dari `app/globals.css`; tidak memakai gradient.

Derived presentation data seperti seat orientation, auction rows, compact score labels, dan card layout dibuat di presentation/view-model boundary. React component tidak boleh menjadi engine bridge kedua.

### Tidak termasuk

- Menulis ulang engine, protocol, persistence, atau state machine.
- Chat, spectator, alert/explanation, traveller, review, comparison, MP/IMP, atau result history yang backend-nya belum tersedia. Claim dan undo mengikuti capability backend consensus yang sekarang tersedia.
- Menyalin logo, trademark, artwork, atau branding BBO.
- Menambah sidebar dashboard, marketing header, decorative panel, atau fitur backend hanya untuk mengisi layout.
- Mengimplementasikan UI pada task planning ini.

## 3. Reference gate

Panduan teks BBO sudah diterima. Paket referensi yang tersedia saat penyusunan plan hanya berisi `pasted-text.txt`; tidak ada file screenshot di dalam attachment tersebut.

Sebelum Work 4 dimulai, implementer wajib:

- memperoleh atau menerima screenshot BBO untuk AUCTION, PLAY, dummy revealed, current trick, SCORE, dan mobile;
- menyimpan reference checklist tanpa menyalin branded asset;
- membandingkan struktur side-by-side pada viewport desktop dan mobile;
- memakai pertanyaan kelulusan: “Jika warna dan font dihilangkan, apakah struktur spasialnya tetap jelas berasal dari pola meja BBO?”

Jika jawabannya tidak, layout belum dapat diterima meskipun terlihat bersih atau modern.

## 4. Audit implementasi saat ini

### Masalah struktural

- `app/page.tsx` menaruh identity, lobby, dan table di bawah hero/status marketing pada satu document flow.
- Guest login hanya mengubah conditional render; tidak ada redirect atau halaman lobby terpisah.
- Active table masih berada di dalam `.site-shell`, header, footer, dan spacing halaman biasa.
- `.table-layout` memakai `.table-sidebar` + content sehingga terlihat seperti dashboard.
- `.seat-map` adalah visualisasi kursi besar, tetapi auction/play/result tetap ditumpuk sebagai panel berbeda di bawahnya.
- `.table-toolbar h2` dan heading phase terlalu besar untuk game client.
- `.auction-history` berbentuk kumpulan item, bukan grid auction W/N/E/S.
- `.bid-builder` memakai `<select>` level + `<select>` strain + tombol Bid, bukan bidding box.
- `Hand` memakai suit row dengan tombol rank kecil; tidak terbaca sebagai kartu fisik.
- `.current-trick` menampilkan kartu horizontal sehingga tidak menunjukkan asal seat secara spasial.
- Dummy tampil sebagai panel terpisah, bukan bagian geometri meja.
- Result menjadi panel besar setelah play, bukan state ringkas dari shell yang sama.
- Mobile stylesheet terutama menumpuk desktop blocks menjadi satu kolom; mental model meja bridge tidak dipertahankan.
- Error client direduksi menjadi satu `message: string`, sehingga source, severity, retryability, dan tindakan lanjut hilang.
- Error `STALE_CONTROLLER`/`STATE_CHANGED` tidak memicu resync projection; takeover dapat terus menggunakan revision/controller epoch lama.
- Takeover tidak mempunyai pending/success state yang menjelaskan apakah controller sudah berpindah.
- Debug output atau guard sementara di active table tidak boleh tersisa pada implementation final.

### Keputusan terhadap implementasi yang ada

| Existing item | Keputusan | Arah perubahan |
| --- | --- | --- |
| `app/page.tsx` | Refactor | Landing dan guest entry saja; hilangkan mounted lobby/table dari landing. |
| `useTableSession` credential storage/refresh | Pertahankan lalu rapikan | Pertahankan long-lived device identity, tab access token, ticket, generation fence, resume, dan no mutation retry. Pisahkan route navigation dan typed issue presentation dari transport. |
| `table-state.ts` stale-room/sequence reducer | Pertahankan dan perluas | Pertahankan private-state clearing dan monotonic sequence; tambah typed issue dan explicit resync/takeover state. |
| `BridgeTable` monolith | Refactor bertahap | Jadikan orchestrator tipis di dedicated table route; ekstrak hanya unit visual/behavioral yang bermakna. |
| `SeatPosition` | Refactor | Menjadi player position yang melekat pada geometri shell dan dapat memakai viewer-relative orientation. |
| `Hand` | Replace | Ganti dengan `BridgeHand` + reusable `PlayingCard`. |
| `.table-sidebar` | Delete dari active gameplay | Waiting-only actions dapat masuk compact drawer/popover/status region; player identity tetap pada posisi meja. |
| `.seat-map` / `.table-center` | Replace | Ganti dengan persistent `BridgeTableShell` dan `PlaySurface`. |
| `.auction-panel` / `.auction-actions` / `.bid-builder` | Replace | Ganti grid auction W/N/E/S dan direct-action `BiddingBox`. Tidak ada primary `<select>`. |
| `.play-panel` / `.card-hand` | Replace | Own hand di bottom edge, dummy pada posisi meja, kartu fisik dan langsung playable. |
| `.current-trick` horizontal | Replace | Empat slot N/E/S/W di pusat meja. |
| `.result-panel` | Refactor | Compact in-shell board result dengan geometry shell tetap stabil. |
| `.connection-status` | Pertahankan dan padatkan | Masuk status strip/toast; jangan mengambil area utama. |
| `playableHand()` | Pertahankan | Tetap presentation guard untuk own/dummy/follow-suit; server tetap validator final. |
| owner/waiting commands | Pertahankan dan perluas | Susun ulang sebagai compact controls; empty-seat portal dapat menambah bot, sedangkan player portal dapat mengeluarkan atau mengganti non-owner dengan bot. |

## 5. Target route dan navigation architecture

### `/` — landing dan guest entry

- Marketing hero boleh tetap berada di landing.
- Form guest identity berada di landing atau dibuka dari CTA yang jelas.
- Setelah `createIdentity()` berhasil dan credential sudah dipersist, jalankan `router.replace("/lobby")`.
- Kegagalan identity tidak melakukan redirect dan fokus dipindahkan ke error summary/form field yang relevan.
- Guest yang sudah valid dan sengaja membuka `/` boleh melihat CTA `Lanjut ke lobby`; jangan memindahkan tanpa konteks ketika pengguna hanya melihat landing.

### `/lobby` — halaman baru setelah masuk sebagai tamu

- Hanya memakai navbar aplikasi yang compact; tidak ada hero, runtime marketing section, atau footer besar.
- `<h1>` terbesar di halaman adalah sapaan nama guest: `Halo, {nickname}`.
- Create table dan join by invite menjadi aksi utama di bawah sapaan.
- Navbar minimum: wordmark/home, status identity singkat, dan action ganti nama/keluar.
- Invite query tetap dapat dibaca, dinormalisasi, dan dipertahankan saat error.
- Tanpa guest identity yang dapat dipulihkan, route guard melakukan `replace("/")` setelah initialization selesai.
- Bila stored active table valid, tampilkan explicit `Lanjutkan meja` atau arahkan ke canonical table route setelah projection berhasil dipulihkan; jangan menampilkan private table state sesaat di lobby.

### `/table/[tableId]` — dedicated bridge client

- Route ini tidak memakai marketing hero, normal site footer, atau dashboard navbar.
- Waiting table boleh mempunyai compact invite/seat/owner controls.
- Saat `ACTIVE`/`BETWEEN_BOARDS`, bridge table menggunakan viewport mendekati `100dvh` dan menjadi aplikasi utama.
- Canonical table ID di URL harus sama dengan active projection. Mismatch memicu clear/unsubscribe sebelum table lain dimuat.
- Keluar/finish menghapus private client state sebelum kembali ke `/lobby`.
- Invite URL baru mengarah ke `/lobby?invite=CODE`, bukan ke anchor di landing.

## 6. Typed error and recovery UX

Ganti `message: string | null` sebagai satu-satunya error representation dengan typed client issue, misalnya semantic shape:

```ts
type ClientIssue = {
  kind: "notFound" | "unavailable" | "full" | "locked" | "offline" | "network" | "timeout" | "session" | "conflict" | "server" | "validation";
  title: string;
  detail: string;
  retryable: boolean;
  action?: "retry" | "editInvite" | "backToLobby" | "signInAgain" | "resync" | "takeover";
  source: "rest" | "websocket" | "browser";
};
```

Nama final boleh menyesuaikan existing convention. Jangan menggunakan HTTP status saja: prioritaskan stable problem `code`, lalu status/network classification sebagai fallback.

### Error matrix minimum

| Kondisi | Pesan utama | Detail yang membantu | Action |
| --- | --- | --- | --- |
| `TABLE_NOT_FOUND` / invite salah | **Meja tidak ditemukan** | Kode mungkin salah atau meja sudah dihapus. | Fokus/edit kode, kembali ke lobby. |
| `TABLE_UNAVAILABLE` | **Meja sudah tidak tersedia** | Meja mungkin sudah dimulai, selesai, atau undangan tidak berlaku. | Kembali ke lobby; jangan retry otomatis. |
| `TABLE_FULL` | **Meja sudah penuh** | Empat pemain sudah bergabung. | Kembali atau gunakan kode lain. |
| `TABLE_LOCKED` | **Meja sedang dikunci** | Pemilik belum membuka meja untuk pemain baru. | Retry manual setelah konfirmasi pemilik. |
| invalid invite format | **Kode undangan tidak valid** | Jelaskan format tanpa melakukan request yang pasti gagal. | Edit kode. |
| browser offline | **Kamu sedang offline** | State committed terakhir tetap terlihat, mutation disabled. | Otomatis reconnect saat online; retry manual tersedia. |
| fetch gagal/DNS/CORS/API down | **Tidak dapat menghubungi server** | Bedakan dari table not found; data meja belum dapat diperiksa. | Retry manual. |
| timeout | **Server terlalu lama merespons** | Permintaan belum dipastikan berhasil. | Resync sebelum mutation baru; jangan retry mutation otomatis. |
| `SERVICE_UNAVAILABLE`/5xx | **Layanan meja sedang bermasalah** | Gunakan `retryable` dari Problem Details bila ada. | Retry manual. |
| session inactive/invalid | **Sesi tamu sudah berakhir** | Private table state dibersihkan. | Masuk kembali. |
| `STATE_CHANGED` | **Meja sudah berubah** | Projection terbaru harus dimuat sebelum command berikutnya. | Resync. |
| `STALE_CONTROLLER` | **Kendali ada di perangkat lain** | Jelaskan bahwa state perlu diselaraskan sebelum takeover. | Resync lalu Ambil alih. |
| illegal call/card/turn | Pesan aksi spesifik | Pertahankan meja dan state committed; jangan ubah revision lokal. | Koreksi pilihan. |

Presentation rules:

- Lobby errors tampil dekat join/create action dan diumumkan dengan `role="alert"`.
- Connection degradation tampil di compact persistent status; jangan menggantikan seluruh table.
- Command rejection tampil sebagai compact table toast/notice tanpa menggeser geometri meja.
- Setiap issue hanya menawarkan action yang aman. Mutation yang outcome-nya belum diketahui tidak boleh dikirim ulang otomatis.
- Raw server title, internal error, request ID, token, credential, table secret, atau hidden card tidak ditampilkan/log ke browser console.
- Tambahkan unit tests untuk pemetaan Problem/WS/browser error sehingga `TABLE_NOT_FOUND`, full, unavailable, dan network failure tidak pernah menjadi pesan yang sama.

Catatan contract: audit join endpoint harus membuktikan `TABLE_FULL`/`TABLE_LOCKED` tetap keluar sebagai stable public code. Jika repository saat ini meratakan kondisi tersebut menjadi `TABLE_UNAVAILABLE` atau 500, buat task contract kecil dan kompatibel sebelum wiring UI; jangan menebak dari string title.

## 7. Controller takeover recovery

### Masalah saat ini

`sendCommand("table.takeover")` memakai `expected_revision` dan `controller_epoch` dari projection lokal. Saat server mengembalikan `STALE_CONTROLLER` atau `STATE_CHANGED`, client hanya menghapus pending request dan memasang message. Projection/revision/epoch tidak diperbarui, sehingga klik berikutnya dapat mengulang stale command yang sama. Selain itu tidak ada visual success state setelah `CONTROLLER_REPLACED`.

### Flow yang wajib

1. Deteksi `STALE_CONTROLLER` atau `STATE_CHANGED` dari WS error.
2. Bekukan game mutations dari tab tersebut dan tandai controller state `stale`.
3. Minta resume/snapshot untuk active table menggunakan sequence committed terakhir; jangan retry mutation yang ditolak.
4. Setelah projection fresh diterima, tampilkan action **Ambil alih kendali** dengan penjelasan singkat.
5. Kirim satu `table.takeover` memakai revision dan controller epoch dari projection fresh.
6. Pertahankan pending state sampai event projection yang memuat `CONTROLLER_REPLACED`/epoch baru diterima.
7. Tampilkan success feedback `Kendali sudah berpindah ke perangkat ini`, kembalikan controller state ke `current`, dan aktifkan legal mutations.
8. Error/reconnect di tengah takeover kembali ke langkah resync; tidak ada automatic takeover retry.

Button rules:

- jangan tampil permanen tanpa konteks pada semua seated player;
- tampilkan ketika conflict terdeteksi atau melalui compact secondary device-control menu;
- disabled selama `syncing`, `offline`, atau takeover pending;
- click berulang tidak membuat request paralel;
- keyboard focus tetap pada feedback/action yang relevan;
- current controller dan stale controller harus mempunyai status yang dapat dipahami tanpa melihat log.

Tests minimum:

- reducer: stale error → resyncing → fresh snapshot → takeover pending → replaced event → current;
- stale revision tidak dikirim ulang;
- two-tab scenario untuk session/seat yang sama membuktikan controller epoch bertambah dan tab peminta dapat kembali melakukan command;
- failure di setiap tahap menghasilkan action recovery yang benar;
- tidak ada stale pending/controller/private state setelah pindah table.

Jika test membuktikan contract server tidak memungkinkan takeover setelah fresh projection, lakukan diagnosis contract terarah. Perubahan backend hanya boleh berupa koreksi bug kompatibel; jangan mendesain ulang controller fencing atau protocol.

## 8. Persistent BridgeTableShell

AUCTION, OPENING_LEAD/PLAY, BOARD_SCORED, dan BETWEEN_BOARDS adalah state dari shell yang sama, bukan halaman/panel berbeda.

```text
┌──────────────────────────────────────────────────────────────┐
│ TableStatusBar: board · dealer · vul · contract · tricks · ● │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│                    MAIN GAME SURFACE                         │
│       North/partner · West · center trick · East             │
│       phase content occupies stable table geometry           │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ contextual controls / local player hand anchored at bottom   │
└──────────────────────────────────────────────────────────────┘
```

Shell rules:

- geometry luar stabil ketika AUCTION → PLAY → SCORE → NEXT BOARD;
- bridge surface mendominasi viewport;
- board, dealer, vulnerability, contract, declarer, tricks, dan connection masuk satu compact status strip;
- nama/seat player melekat pada posisi N/E/S/W, bukan participant sidebar;
- invite, settings, leave, finish, dan owner actions memakai ruang minimum dan hanya muncul saat relevan;
- tidak ada giant `Board 1` heading, nested cards, marketing header, atau document scroll untuk action normal pada desktop umum;
- waiting room boleh berbeda secara internal, tetapi transisi ke active shell tidak membawa dashboard sidebar.

Gunakan derived `TableOrientation` agar local player dapat ditempatkan konsisten di sisi bawah seperti bridge client, sambil tetap menampilkan actual seat/dealer/declarer secara benar. Orientation hanya mapping visual; projection dan command tetap memakai seat domain N/E/S/W. Auction table tetap mempertahankan urutan bridge yang dapat dibaca, default W/N/E/S.

## 9. Auction UX

### Auction table

- Gunakan semantic table/grid dengan kolom `W | N | E | S`.
- Letakkan call secara kronologis pada kolom caller; slot sebelum dealer pada row pertama kosong.
- Pass, bid, X, dan XX dibaca sebagai call bridge, bukan chip/card terpisah.
- Dealer, vulnerability, dan current bidder terlihat dekat grid tanpa banner besar.
- Current bidder memakai highlight/focus yang jelas tetapi restrained.
- Local hand tetap terlihat sepanjang auction.

### BiddingBox

Primary interaction harus direct dan minimum-action:

```text
[ PASS ] [ X ] [ XX ]
[  1  ][  2  ][  3  ][  4  ][  5  ][  6  ][  7  ]
[  ♣  ][  ♦  ][  ♥  ][  ♠  ][ NT  ]
```

- Hapus `<select>` level/strain dan tombol form-style Bid dari primary UX.
- Pemilihan level lalu strain (atau direct bid matrix bila reference menunjukkan itu) mengirim satu call dengan interaction cost rendah.
- Illegal call disabled atau absent berdasarkan engine-authoritative legality.
- Server tetap validator final; React tidak boleh menyalin seluruh auction engine.
- Audit projection sebelum implementation. Jika current projection belum dapat menyatakan legal calls, gunakan additive, recipient-safe presentation field yang dihitung dari engine sebagai dependency terkecil; jangan mengubah envelope/state-machine architecture dan jangan membuat legality kedua di TypeScript.
- Pass/Double/Redouble hanya aktif pada turn yang benar dan saat call tersebut legal.
- Shortcut keyboard memiliki label/help, tidak aktif ketika focus berada di input, dan tidak menggantikan touch controls.
- Pending call mengunci duplicate submit sampai event/error authoritative diterima.

## 10. Card dan hand rendering

Gunakan reusable `PlayingCard` yang cukup bermakna untuk dipakai pada hand, dummy, dan trick tanpa membuat atom abstraction berlebihan.

Required variants:

- hand card;
- dummy card;
- trick card;
- disabled/illegal;
- selected/hover/focus;
- compact mobile.

Setiap kartu menampilkan rank dan suit dengan red/black distinction, outline/focus yang jelas, dan proporsi kartu fisik. Jangan gunakan tombol rank-only generik.

`BridgeHand` requirements:

- own hand anchored pada bottom edge selama auction dan play;
- horizontal overlap/fan yang tetap memungkinkan setiap rank/suit dibaca di desktop;
- layout compact seperti hand bridge pada mobile, tanpa horizontal page overflow;
- playable cards mempunyai affordance yang jelas;
- illegal cards disabled secara semantic dan tidak mengirim command;
- follow-suit/dummy source tetap memakai derived state yang ada dan server validation final;
- hidden opponent hand tidak dirender dari data yang tidak tersedia; jangan membuat placeholder yang membocorkan count/detail di luar contract.

## 11. Play surface

Required spatial composition:

```text
                       NORTH / PARTNER
                       [dummy when visible]

WEST / LHO             [N trick card]             EAST / RHO
                       [W] [center] [E]
                       [S trick card]

                       SOUTH / YOU
                       [own hand]
```

- Gunakan flat table/felt-like token yang harmonis dengan palette; tidak ada gradient.
- Player label tetap melekat pada posisi meja dan menunjukkan turn secara compact.
- Dummy muncul di posisi partner/dummy yang benar sebagai bagian surface, bukan generic card panel.
- Current trick mempunyai empat spatial slot N/E/S/W; lokasi kartu menjelaskan siapa yang memainkan.
- Completed trick boleh memakai motion singkat lalu hilang, dengan `prefers-reduced-motion` fallback.
- Own hand dapat dimainkan tanpa scroll dan mempunyai touch target yang memadai.
- Declarer dapat memainkan dummy pada turn dummy; defender/dummy tidak memperoleh control yang dilarang.
- Jangan pernah membuat semua tangan dari `fullDeal` sebelum BOARD_SCORED; rendering mengikuti recipient projection saja.

## 12. Score dan next board

Board result tetap berada di shell yang sama dan mempertahankan status/player/table geometry.

Prioritas informasi:

1. board number;
2. final contract dan doubled/redoubled state;
3. declarer;
4. made/exact, `+N`, atau `-N`;
5. tricks;
6. NS score dan EW score bila dapat diturunkan tanpa ambiguity;
7. vulnerability.

Actions compact dan hanya yang didukung:

- `Board berikutnya` untuk owner pada BETWEEN_BOARDS;
- `Akhiri meja` bila legal;
- `Kembali ke lobby` setelah FINISHED.

Jangan menampilkan Results, Review, Traveller, atau comparison sebelum capability backend tersedia. Claim dan undo ditempatkan sebagai kontrol kontekstual tanpa mengubah geometri shell. Susunan result boleh menyediakan extension region kecil agar fitur duplicate comparison masa depan tidak memerlukan shell baru.

## 13. Responsive strategy

### Desktop

- Target gameplay surface `min-height: 100dvh`/available viewport setelah compact status bar.
- Status, players, auction/play controls, current trick, dan own hand terlihat tanpa document scrolling pada 1280×720 dan 1440×900.
- Optional contextual region harus kecil dan collapsible; tidak boleh menghidupkan kembali sidebar besar.

### Tablet

- Pertahankan N/E/S/W geometry.
- Kompres card size, label, gap, dan control density sebelum mengubah composition.
- Landscape dan portrait tetap mempunyai own hand yang accessible.

### Mobile

Auction order:

```text
[compact status]
[auction grid]
[PASS / X / XX]
[levels 1–7]
[♣ ♦ ♥ ♠ NT]
[own hand]
```

Play order:

```text
[compact status]
[dummy]
[N/E/S/W table + spatial trick]
[own hand]
```

- Jangan menumpuk seluruh desktop dashboard menjadi daftar panel.
- Gunakan `100dvh` dan safe-area inset untuk mobile browser chrome.
- Card tetap readable/tappable; own hand dan action penting tidak tertutup virtual keyboard.
- Internal region boleh scroll hanya bila viewport ekstrem; normal gameplay tidak mengandalkan page scroll.

## 14. Component architecture

Ekstrak hanya unit yang mempunyai visual/behavioral responsibility nyata:

```text
LobbyPage
└── lobby create/join actions

TablePage
└── BridgeTableShell
    ├── TableStatusBar
    ├── PlayerPosition (reused N/E/S/W)
    ├── AuctionTable
    ├── BiddingBox
    ├── PlaySurface
    │   ├── CurrentTrick
    │   ├── BridgeHand
    │   └── DummyHand
    ├── BoardResult
    └── compact notice/secondary actions

BridgeHand
└── PlayingCard (reused card instances)
```

`DummyHand` boleh menjadi variant/composition dari `BridgeHand` bila tidak mempunyai behavior independen. Jangan memecah heading, button, suit symbol, atau setiap status value menjadi component kecil.

Presentation helpers yang layak diuji:

- actual-seat ↔ visual-position orientation;
- auction calls → W/N/E/S row matrix;
- result → made/plus/minus label;
- Problem/WS/browser failure → `ClientIssue`;
- playable card key/source yang sudah ada.

## 15. Files likely affected

| File | Planned change |
| --- | --- |
| `app/page.tsx` | Landing + guest entry saja; redirect sukses ke `/lobby`. |
| `app/lobby/page.tsx` | New guest lobby route dengan navbar ringkas dan greeting `<h1>` terbesar. |
| `app/table/[tableId]/page.tsx` | New dedicated table route dan route boundary. |
| `app/bridge-table.tsx` | Diperkecil atau dipindah menjadi shell orchestrator; phase panels lama dihapus. |
| `app/use-table-session.ts` | Typed issue, explicit resync/takeover lifecycle, route-safe enter/clear, dan no-retry preservation. |
| `app/table-state.ts` | Controller/resync state, typed issue, orientation/phase presentation boundary bila tepat. |
| `app/globals.css` | Hapus dashboard selectors; tambah route shell, BBO geometry, physical card, bidding box, spatial trick, responsive/safe-area rules. |
| `app/layout.tsx` | Pertahankan metadata/global document; jangan membuat seluruh app Client Component. |
| Focused test files | Error mapping, reducer, auction matrix, orientation, takeover, keyboard, responsive/E2E. |

Nama/path final boleh mengikuti struktur saat implementasi setelah audit, tetapi tidak boleh memperkenalkan component library atau styling system kedua.

## 16. Migration sequence

### Work 0 — evidence and contract audit

- [x] Terima/inspect screenshot BBO desktop/mobile untuk seluruh phase.
Screenshot sudah tersedia di /docs/*.png
- [x] Catat current projection field yang dibutuhkan tiap visual region.
- [x] Audit stable REST/WS code untuk not-found/full/locked/unavailable/network/session.
- [ ] Reproduce takeover failure dengan dua tab dan catat frame revision/seq/controller epoch.
- [x] Pastikan tidak ada hidden-state requirement baru.

### Work 1 — route and guest lobby boundary

- [x] Pisahkan `/`, `/lobby`, dan `/table/[tableId]`.
- [x] Redirect guest-login success ke `/lobby`.
- [x] Buat compact lobby navbar tanpa hero.
- [x] Jadikan `Halo, {nickname}` heading paling besar.
- [x] Tambah route guards, invite preservation, active-table continuation, dan private-state clearing.

### Work 2 — typed errors and takeover correctness

- [x] Implement stable error classifier dan contextual recovery UI.
- [x] Bedakan not found, unavailable, full, locked, offline, network, timeout, session, conflict, dan server failure.
- [x] Implement stale-controller resync → fresh projection → explicit takeover → success event flow.
- [x] Tambah reducer tests dan pertahankan no automatic mutation retry.

### Work 3 — persistent shell and presentation model

- [x] Buat `BridgeTableShell`, compact `TableStatusBar`, dan viewer-relative orientation.
- [x] Pindahkan player identity ke N/E/S/W table positions.
- [x] Hilangkan active `.table-sidebar`, giant headings, dan stacked phase geometry.
- [x] Pastikan outer geometry stabil lintas phase.

### Work 4 — physical cards and auction

- [x] Implement `PlayingCard` dan `BridgeHand`.
- [x] Buat auction grid W/N/E/S dengan dealer offset.
- [x] Ganti select-based bid builder dengan direct BBO-style `BiddingBox`.
- [x] Hubungkan legal/pending/keyboard/touch state tanpa menduplikasi engine.
- [x] Jaga own hand selalu terlihat.

### Work 5 — play, dummy, trick, score

- [x] Buat central `PlaySurface`.
- [x] Tempatkan dummy dalam geometri meja.
- [x] Tempatkan current trick secara spasial N/E/S/W.
- [x] Integrasikan own-hand bottom anchor dan declarer dummy controls.
- [x] Masukkan compact `BoardResult` dan next/finish ke shell yang sama.

### Work 6 — responsive, accessibility, cleanup

- [x] Implement viewport-bound desktop gameplay tanpa normal document scroll.
- [x] Pertahankan geometry pada tablet melalui responsive compression.
- [x] Implement mobile auction/play composition, safe areas, dan touch sizing.
- [x] Tambah semantic table/button, focus-visible, live region, keyboard help, dan reduced motion.
- [x] Hapus obsolete selectors/components/debug output dan dead phase UI.

### Work 7 — regression and visual gate

- [x] Unit tests untuk presentation transforms, error mapping, reducer, legal card source, dan takeover lifecycle.
- [ ] Browser tests untuk route redirect/guard, error recovery, auction controls, play, score, next board, dan reconnect.
- [ ] Four-context happy path create → join → seat → auction → play → score → next/finish.
- [ ] Two-tab takeover dan stale command rejection.
- [ ] Network assertion membuktikan hidden hand tidak bocor.
- [ ] Side-by-side screenshot validation terhadap BBO reference pada desktop/tablet/mobile.
- [x] Lint, strict typecheck, production build, dan reduced-motion implementation pass.

## 17. Acceptance criteria

### Guest dan lobby

- Menekan **Masuk sebagai tamu** dan menerima response sukses selalu berpindah ke `/lobby`.
- `/lobby` tidak mempunyai hero marketing; hanya navbar ringkas dan application content.
- Sapaan yang memuat nickname guest adalah `<h1>` dan heading visual terbesar.
- Refresh `/lobby` memulihkan identity tanpa flash private table content.
- Guest invalid diarahkan kembali ke `/` dan private state dibersihkan.

### Error dan recovery

- Table not found, table unavailable, table full, table locked, offline, network failure, timeout, session expiry, dan server failure mempunyai copy serta action yang berbeda.
- Network error tidak pernah ditampilkan sebagai table not found.
- Invite tetap ada di input setelah join gagal.
- Retry hanya tersedia untuk operasi yang aman; mutation tidak diulang otomatis.
- Error dapat dibaca screen reader dan tidak menggeser geometry active table secara besar.

### Takeover

- Tombol takeover tidak mengirim revision/controller epoch stale berulang kali.
- `STALE_CONTROLLER` menyebabkan resync projection sebelum takeover baru boleh dikirim.
- Satu click setelah fresh projection menghasilkan tepat satu takeover request.
- Event authoritative mengubah UI menjadi `Kendali sudah berpindah ke perangkat ini`.
- Tab peminta dapat kembali melakukan call/play yang legal.
- Pending controller state bersih setelah reconnect, table switch, leave, atau finish.

### General table

- Active gameplay tidak lagi terlihat seperti SaaS/dashboard.
- Bridge table mendominasi viewport.
- AUCTION, PLAY, dan SCORE memakai satu visual shell.
- Normal gameplay tidak memerlukan page scrolling pada desktop umum.
- Game-engine behavior dan server-authoritative rules tidak berubah.
- Hidden hands tetap terlindungi pada DOM, projection, network assertion, error, dan logs.
- Kursi kosong membuka portal dengan aksi duduk dan, untuk owner, tambah bot; player non-owner mempunyai aksi terpisah untuk keluarkan dan keluarkan lalu ganti bot.

### Auction

- Pemain BBO langsung memahami auction history, current bidder, Pass, Double, Redouble, levels 1–7, suits/NT, dan lokasi kartunya tanpa penjelasan.
- Auction history berperilaku sebagai auction table W/N/E/S.
- Tidak ada level/strain `<select>` pada primary bidding interaction.
- Illegal calls disabled/absent berdasarkan engine-authoritative legality.
- Keyboard dan touch menghasilkan command yang sama dan tidak dapat duplicate-submit.

### Play

- Pemain langsung mengenali North, East, South, West, dummy, own hand, current trick, dan player on turn.
- Setiap current-trick card berada di slot seat yang memainkannya.
- Own hand tetap accessible di bottom edge.
- Dummy berada pada virtual table, bukan generic panel.
- Illegal/follow-suit card tidak dapat mengirim command.

### Score

- Contract, declarer, doubled/redoubled state, made/plus/minus result, tricks, score, vulnerability, dan board number dapat dipahami dalam beberapa detik.
- PLAY → RESULT → NEXT BOARD terasa tetap berada di meja yang sama.
- Hanya action yang didukung backend yang ditampilkan.

### Responsive dan accessibility

- Mobile auction controls langsung usable; card readable/tappable; own hand terlihat.
- Current trick dan N/E/S/W tetap dapat dipahami pada viewport sempit.
- Tidak terjadi generic desktop-dashboard stacking.
- Semua action memakai button/link semantic, focus-visible, keyboard path, dan label screen-reader yang jelas.
- Motion bersifat cepat/subtle dan dihentikan oleh `prefers-reduced-motion`.

### Visual fidelity gate

- Struktur dinilai side-by-side dengan screenshot BBO, bukan berdasarkan “terlihat modern”.
- Jika warna/font dinetralkan, table composition tetap jelas mengikuti pola BBO.
- Target akhir sekitar 90% BBO UX/layout fidelity dan 10% BridgeYok modernization.

## 18. Definition of done

Roadmap ini selesai hanya ketika seluruh acceptance criteria terbukti dengan test/evidence, obsolete dashboard/table UI sudah dihapus, dan four-client phase gate tetap lulus. Completion tidak boleh dinyatakan hanya karena screenshot terlihat mirip.

Urutan prioritas keputusan selama implementasi:

1. Correct bridge information.
2. BBO structural familiarity.
3. Fast gameplay.
4. Visibility of local hand.
5. Visibility of auction/current trick.
6. Minimal interaction cost.
7. Responsive usability.
8. Accessibility.
9. Visual polish.

Visual novelty bukan tujuan.

---

## 19. Objective GUX — Gameplay UX Reliability & Interaction Refactor

**Status:** PLANNED / NOT STARTED

**Priority:** UX correctness dan gameplay feel kira-kira 10× lebih penting daripada decorative polish.

**Character:** frontend-first, regression-sensitive, mengikuti convention meja bridge/BBO yang sudah disetujui, dan bergantung pada realtime core serta pure bridge engine yang ada.

Objective ini memperbaiki feedback aksi yang terlambat, batas component yang terlalu besar, presentasi kartu yang mudah drift, spatial collision, tidak adanya motion/drag/audio fungsional, dan control/copy aktif yang terlalu berat. Ini bukan redesign aturan bridge, izin untuk mengubah protocol secara serampangan, atau alasan menulis ulang pure Go engine. Server dan PostgreSQL tetap authoritative; frontend hanya membuat projection legal lokal terasa segera.

### 19.1 Audit baseline 31 Agustus 2026

- `app/bridge-table.tsx` memuat `PlayingCard`, `BridgeHand`, `PlayerPosition` + portal, `AuctionTable`, `BiddingBox`, `CurrentTrick`, `ConsensusControls`, `TableSurface`, `WaitingRoom`, `BoardResult`, dan route orchestrator dalam satu file sekitar 1.300 baris. Ada domain yang layak diekstrak, tetapi wrapper JSX sederhana tidak boleh dijadikan component baru.
- Satu `PlayingCard` sudah dipakai untuk own hand, dummy, dan trick, namun API hanya mempunyai `variant`, `disabled`, dan `playable`; sizing/layout masih tersebar di selector context. Refactor harus mempertahankan satu primitive ini dan memperluas contract berdasarkan kebutuhan nyata, bukan membuat tiga renderer.
- `playableHand()` menghitung source own/dummy dan follow-suit legal cards. Auction memakai `game.legalCalls`. Server tetap memvalidasi seluruh call/card. Claim/undo capability datang dari projection, tetapi beberapa known-invalid action masih berakhir sebagai generic issue.
- `useTableSession.sendCommand()` membuat `request_id`, memakai projection `revision` dan `controller_epoch`, lalu hanya menyimpan `requestId → commandName`. Tidak ada optimistic payload/base revision/projection. Semua mutation juga diblok ketika command mana pun pending.
- Server mengirim accepted ACK dengan `request_id`, revision, dan seq, lalu recipient-specific event projection. Client saat ini tidak menangani ACK; reducer mengganti seluruh table projection dan menghapus seluruh pending map pada setiap event/snapshot. Event projection tidak membawa originating request ID.
- Conflict `STALE_CONTROLLER`, `STATE_CHANGED`, atau `REVISION_CONFLICT` membersihkan pending, memicu resume, dan snapshot menjadi recovery path. Disconnect juga membersihkan pending. Desain optimistic harus menjaga fencing/no automatic stale intent retry ini.
- Authoritative `ProjectedGame` saat ini sudah membawa public `CompletedTricks` untuk semua participant, termasuk current trick, turn, counts, own hand, revealed dummy, dan full deal hanya setelah score. ENG-01 wajib mempersempit presentation/projection sesuai information policy, bukan menganggap field yang sudah terkirim otomatis boleh diekspos penuh.
- Tidak ada library motion, drag/drop, icon, audio, state, atau toast tambahan. Runtime web hanya Next/React; CSS hanya mempunyai transition umum/loading spin dan reduced-motion override. Pointer Events, Web Audio/HTMLAudio, CSS/WAAPI, serta React primitives harus diaudit dahulu sebelum dependency baru dipertimbangkan.
- Unit test berbasis `node:test` mencakup projection normalization, reducer, request ID, issue mapping, orientation/auction/legal cards. Playwright/config dan satu four-context desktop flow ada sebagai worktree yang belum selesai; belum ada mobile/touch, screenshot matrix, delayed ACK/rejection, optimistic reconnect, atau animation-order coverage.

### UX-01 — Frontend component architecture

- **Problem:** gameplay domains dan interaction boundaries menumpuk di `bridge-table.tsx`, sehingga card layout, portal, game action, dan phase rendering sulit diuji atau diubah tanpa regresi.
- **Scope:** behavior-preserving extraction untuk canonical `PlayingCard`; `BridgeHand`/`OwnHand`; `DummyHand`, `DummySuit`, dan layout orientation; `CurrentTrick`/played card/result indicator; auction/bidding; participant identity/position/bot marker/portal; shared dialog primitive bila benar-benar reused; status/navbar/actions; contract/invite; sound, drag, optimistic reconciliation, dan presentation queue boundaries. Audit nama final terhadap code sebelum membuat file.
- **Non-goals:** tidak memecah setiap element/wrapper, tidak mengubah rules/protocol/layout behavior pada extraction commit, dan tidak membuat near-duplicate card atau hand components.
- **Affected:** `bridge-table.tsx`, `table-state.ts`, `use-table-session.ts`, focused component/module files di `app/table` atau struktur existing terdekat, dan gameplay CSS/tests.
- **Architecture:** visual components menerima derived props; legality/realtime/reconciliation tidak diduplikasi di renderer. Own, dummy, dan trick wajib memakai satu card primitive dengan context variants/tokens untuk orientation, size, playable, selected, disabled, dragging, played, dummy, interaction mode, dan emphasis hanya sejauh dibutuhkan code.
- **Dependencies:** audit baseline dan passing behavior regression untuk flow existing.
- **Sequence:** lock current behavior tests → extract card/hand → auction/trick → participant/portal → navbar/actions → move interaction state boundaries → delete duplicate markup/selectors.
- **Acceptance:** route orchestrator tidak lagi menjadi dumping ground; setiap extracted unit mempunyai domain/behavior responsibility; one canonical card renderer; duplicate card markup/style hilang; realtime/business logic tidak menyebar ke visual leaves.
- **Browser/mobile:** existing auction/play/waiting/result flow identik sebelum behavioral objectives; keyboard, touch target, dummy orientations, dan screen-reader labels tidak regress pada desktop dan mobile.
- **Regression tests:** component/unit coverage untuk card variants, ranks/suits, disabled/playable state, hand source, auction/trick orientation, portal focus/Escape, plus existing Playwright happy path.
- **Completion gate:** UX-01 PASS hanya setelah behavior-preserving diff direview terpisah, tests pass, dan UX-02 sampai UX-13 belum dicampur ke extraction.

### UX-02 — Optimistic gameplay state

- **Problem:** legal local bid/play baru terlihat setelah database transaction dan socket event; current global pending lock membuat latency tampak seperti frozen UI.
- **Scope:** explicit authoritative projection, ordered optimistic operations keyed by `request_id` + base revision, projected view, and separate presentation events. Apply legal local call/card immediately; reconcile on ACK/event; rollback/rebase on rejection/conflict; snapshot/resume is ultimate recovery.
- **Non-goals:** server authority tidak berpindah, offline arbitrary intent tidak diantre, animation state tidak menentukan legality, dan tidak membuat satu mutable object untuk authoritative/optimistic/presentation state.
- **Affected:** `use-table-session.ts`, `table-state.ts` or focused optimistic module/hook, realtime envelope handling, auction/hand/trick consumers, and tests. Protocol change is allowed only if repository tests prove ACK revision/seq plus projected events cannot correlate safely.
- **Architecture:** `authoritativeProjection + pendingOptimisticOperations → projectedGameplay`; presentation queue consumes semantic diffs independently. Each operation records payload, request ID, base revision, expected visible effect, and lifecycle. Unrelated confirmed events are applied before rebasing still-valid operations.
- **Dependencies:** UX-01; existing idempotency, revision, seq, controller fencing, and recipient projection contracts.
- **Sequence:** model/reducer tests → handle accepted ACK explicitly → optimistic bid → optimistic own/dummy play → event reconciliation → rejection/conflict rollback/rebase → reconnect/snapshot recovery.
- **Acceptance:** legal play removes card from source and enters pending trick state in the same interaction frame; legal bid appears immediately; confirmation never duplicates it; rejected/conflicting action restores latest valid authoritative view without discarding unrelated newer events.
- **Browser/mobile:** verify with artificial ACK/event latency on desktop and mobile; action feedback begins before network response and remains operable without layout jump.
- **Regression tests:** reducer/model cases for delayed ACK, event-before-ACK, ACK-before-event, rejection, revision conflict, unrelated remote event, duplicate ACK/event, disconnect, reconnect with optimistic work, and snapshot replacement.
- **Completion gate:** UX-02 PASS requires deterministic reconciliation evidence and raw WebSocket assertions; a visually fast animation without rollback/rebase proof is not a pass.

### UX-03 — Legal action prevention

- **Problem:** some predictable rule-invalid actions can still be submitted and surfaced through error UI, conflating bridge legality with infrastructure failure.
- **Scope:** derive legal calls/cards and availability for turn, role, phase, claim, undo, pending consensus, connection, and controller state; prevent/disable known-invalid triggers before the command path. Remove rule-invalidity toasts for conditions already known locally.
- **Non-goals:** client is not the final validator; unexpected network/auth/revision/projection errors remain visible; hidden rules are not reimplemented.
- **Affected:** action selectors/view model, `BiddingBox`, card interaction, navbar claim/undo, participant/table actions, issue mapping.
- **Architecture:** one projected capability layer feeds click/tap/drag and rendered disabled state; server rejection remains defense-in-depth and signals stale/corrupt projection when local capability said legal.
- **Dependencies:** UX-02 projection model and current engine-provided `legalCalls`, follow-suit projection, `canRequestUndo`, action request, seat/turn/role fields.
- **Sequence:** capability matrix → controls wiring → predictable-toast removal → stale-projection diagnostics.
- **Acceptance:** disabled/absent controls cannot dispatch; illegal card/call never reaches `sendCommand`; claim/undo accurately reflect current availability; infrastructure errors retain actionable feedback.
- **Browser/mobile:** keyboard, click, tap, and drag all honor the same disabled state; disabled meaning remains perceivable without relying only on color.
- **Regression tests:** matrix across auction/play/dummy/defender/turn/role/connection/consensus; assert no WS frame for known-invalid attempts and one frame for valid attempts.
- **Completion gate:** UX-03 PASS requires DOM and network-level prevention evidence, not merely server rejection.

### UX-04 — Gameplay motion and pacing

- **Problem:** local/remote cards pop into position and completed tricks disappear immediately, obscuring origin, winner, and action order.
- **Scope:** central interruptible presentation queue for local own/dummy movement, remote movement from participant side, completed-trick pause, winner indication, and collection. Board click completes current normal gameplay animation.
- **Non-goals:** animation never delays logical state/legality, no random skip button, no skip from clicks outside board, no ornamental perpetual motion.
- **Affected:** presentation queue/controller, card/hand/trick/player geometry, board event handler, CSS/WAAPI, reduced-motion policy.
- **Architecture:** queue semantic presentation events against stable layout anchors; authoritative/optimistic state may advance while visual events remain ordered. Skip settles current item to end state and proceeds safely. Reduced motion preserves state cues with minimal duration.
- **Dependencies:** UX-02 semantic reconciliation, UX-06 zones/anchors, canonical card from UX-01.
- **Sequence:** queue unit → local card → dummy card → remote directional card → trick winner/pause/collect → board-only skip → burst/remote ordering.
- **Acceptance:** origin-to-slot movement is visible; four-card trick remains perceptible, winner is indicated, then cards collect; incoming events do not reorder/corrupt presentation; board click finishes current animation only.
- **Browser/mobile:** verify pointer/click skip inside board and non-skip from navbar/dialog; stable at all target viewports and under reduced motion.
- **Regression tests:** queue ordering, skip, rapid remote events, state advance during animation, trick boundary, unmount/reconnect cleanup, reduced-motion behavior.
- **Completion gate:** UX-04 PASS requires recorded browser evidence for local, dummy, remote, completed trick, and skip behavior.

### UX-05 — Card interaction and dragging

- **Problem:** playable cards only support click; desktop/mobile drag has no physical pickup/drop feedback or forgiving drop area.
- **Scope:** Pointer Events interaction for all playable own/dummy cards; entire valid board surface is drop pool; legal release uses the same optimistic play action as click/tap; invalid/cancelled release returns card.
- **Non-goals:** no desktop-only HTML5 drag dependency, no second legality implementation, no tiny drop target, no double command, and no page-scroll regression.
- **Affected:** canonical card, hand/dummy, board surface, focused drag controller/hook, optimistic action dispatcher, pointer CSS.
- **Architecture:** click/tap and pointer-drop converge on one `playCard` capability/command function. Controller owns pointer capture, threshold, cancellation, scroll suppression only during active card drag, and exactly-once completion.
- **Dependencies:** UX-01, UX-02, UX-03, UX-06; implement after stable presentation anchors.
- **Sequence:** controller unit tests → mouse/pen → touch → board drop pool → cancellation/scroll/duplicate safeguards → animation integration.
- **Acceptance:** legal card can be dragged and released anywhere on board to play immediately; illegal/nonplayable card cannot start a play; outside/cancel returns; one gesture emits one command.
- **Browser/mobile:** Chromium desktop plus touch emulation/real mobile viewport; vertical page/hand browsing remains possible when not actively dragging.
- **Regression tests:** click, tap, drag, pointer cancel, lost capture, multi-pointer, scroll, invalid drop, reconnect mid-drag, exactly-one WS frame.
- **Completion gate:** UX-05 PASS requires automated desktop and mobile pointer flows plus manual touch sanity evidence.

### UX-06 — Card scale and board geometry

- **Problem:** own/dummy/trick sizing comes from unrelated context selectors and offsets; dummy/trick/hand collisions remain likely across orientations and narrow widths.
- **Scope:** coherent card proportion/typography tokens and explicit board zones for players, own hand, oriented dummy suit groups, current trick slots, and navbar. Preserve descending rank and suit grouping.
- **Non-goals:** no decorative redesign, no unrelated magic-number patches, no change to bridge orientation rules.
- **Affected:** global card tokens, card/hand/dummy/trick/table CSS, orientation helpers, responsive tests.
- **Architecture:** one card aspect ratio and shared scale levels such as hand/dummy/trick/compact; board grid/zones provide anchors consumed by layout and motion. Top dummy groups horizontal with cards vertical; left/right groups vertical with cards progressing inward.
- **Dependencies:** UX-01; current viewer-relative `tableOrientation()` and `visualPositionForSeat()`.
- **Sequence:** define measurable zones/tokens → card typography → own/trick sizing → three dummy orientations → collision assertions → breakpoint tuning.
- **Acceptance:** larger readable rank/suit where space permits; proportions consistent; dummy never overlaps dummy trick slot; trick never overlaps playable cards; viewer/dummy slots remain separated; left/right trick slots remain biased toward their players.
- **Browser/mobile:** screenshot/geometry checks at wide desktop, laptop, tablet landscape/portrait, common mobile, and 320–400 px widths; browser zoom at 200% remains usable.
- **Regression tests:** bounding-box non-overlap, clipped content, stable aspect ratios, readable rank presence, dummy top/left/right screenshots.
- **Completion gate:** UX-06 PASS requires all target viewport collision assertions and approved screenshots.

### UX-07 — Contract and game information

- **Problem:** active navbar suppresses declarer ownership by formatting contract without declarer identity.
- **Scope:** concise contract + declaring seat + participant name where space permits, using shared formatter/view data.
- **Non-goals:** no rule/scoring change and no explanatory paragraph.
- **Affected:** contract formatter, table facts/status bar, board result consistency, tests.
- **Architecture:** formatter consumes authoritative contract and participant mapping; compact fallback keeps contract and seat even if name does not fit.
- **Dependencies:** current projected contract/declarer and seat assignments.
- **Sequence:** formatter tests → navbar → responsive fallback → result consistency.
- **Acceptance:** active player can identify contract and declarer immediately, e.g. semantic equivalent of `4♠ · N · Mahesa`.
- **Browser/mobile:** name may truncate accessibly, but contract and seat never disappear at target widths.
- **Regression tests:** suits, NT, double/redouble, missing identity, long name, passed-out/no-contract.
- **Completion gate:** UX-07 PASS requires desktop/mobile accessible output and formatter tests.

### UX-08 — Navbar actions

- **Problem:** claim/undo occupy floating board space and availability is tied to a separate consensus panel.
- **Scope:** compact icon + short-label Undo and Claim in table status/navbar; disabled unavailable state; claim trick-count popover/sheet only when needed; pending consensus response remains contextual.
- **Non-goals:** no permanent floating action panel, no excessive helper copy, and no capability invented beyond projection.
- **Affected:** status bar/actions, consensus controls, claim selector/dialog primitive, capabilities.
- **Architecture:** persistent action triggers live in available navbar space; one accessible overlay primitive may be shared with participant portal only if behavior truly aligns.
- **Dependencies:** UX-01, UX-03, current `canRequestUndo`/`actionRequest`; ENG-03 later changes bot consensus semantics, not this placement.
- **Sequence:** move triggers behavior-preservingly → capability state → compact selector → pending response placement → responsive overflow.
- **Acceptance:** Undo/Claim no longer float over gameplay; availability visible and correct; selector keyboard/focus works; board geometry gains space.
- **Browser/mobile:** labels/icons remain actionable on desktop; mobile uses compact accessible treatment without hiding availability.
- **Regression tests:** unavailable/available/pending claim/undo, selector range, exactly-one command, focus return, mobile overflow.
- **Completion gate:** UX-08 PASS requires all consensus interactions through navbar/contextual overlay without board collision.

### UX-09 — Concise gameplay copy

- **Problem:** active-game portal/menu/hints/consensus/waiting transitions contain prose that slows scanning and consumes limited geometry.
- **Scope:** audit player portal, table menu, bidding, claim/undo, bots, invite, hints, and waiting/active transition; retain safety-critical destructive meaning while replacing obvious instruction with icon/short action/tooltip.
- **Non-goals:** do not remove error recovery, destructive clarity, accessible names, or necessary waiting-room onboarding.
- **Affected:** gameplay component strings, accessible labels/tooltips, issue vs rule-feedback mapping.
- **Architecture:** visible copy is compact; assistive labels provide context without duplicating paragraphs visually.
- **Dependencies:** UX-01 and final placement from UX-08/UX-12/UX-13.
- **Sequence:** copy inventory → classify essential/safety/redundant → revise active UI → accessibility review.
- **Acceptance:** no explanatory paragraph for obvious active controls; approved action such as `Keluarkan & ganti bot` remains concise and unambiguous.
- **Browser/mobile:** compact copy neither clips nor forces avoidable wrapping at 320 px; tooltips are not the sole access path on touch.
- **Regression tests:** role/name assertions for critical actions, destructive labels, and no loss of screen-reader semantics.
- **Completion gate:** UX-09 PASS requires copy review across every listed surface, not only navbar.

### UX-10 — Turn audio cues

- **Problem:** auction/play turn transitions can be missed when visual focus is elsewhere.
- **Scope:** short subtle cue only on transition from not-viewer-turn to viewer-turn in auction or play; prevent rerender/resync spam; browser-safe initialization; reasonable mute in existing menu/preferences location.
- **Non-goals:** no background music, arcade effects, repeated cue while state remains true, or autoplay-policy workaround.
- **Affected:** focused sound controller/hook, turn transition selector, table menu mute preference, optional local lightweight audio asset/API.
- **Architecture:** effect keys transition by table/board/phase/turn/revision and records last observed authoritative transition; optimistic state alone must not retrigger. Prefer browser APIs/existing assets before dependency.
- **Dependencies:** UX-02 state layers and stable menu from UX-08/UX-09.
- **Sequence:** transition detector unit → user-gesture audio lifecycle → auction/play cue → resync/reconnect suppression → mute persistence.
- **Acceptance:** one cue on each genuine viewer-turn transition, zero on rerender/same snapshot, muted state respected.
- **Browser/mobile:** verify after user interaction on desktop/mobile browser; denied/suspended audio fails silently without breaking gameplay.
- **Regression tests:** false→true, true→true, true→false→true, phase change, snapshot/reconnect, remote resync, mute, unmount.
- **Completion gate:** UX-10 PASS requires transition tests and browser evidence with audio enabled/muted.

### UX-11 — Trick result visualization

- **Problem:** plain `NS–EW` numbers do not express viewer-relative won/lost tricks or create the intended history entry point.
- **Scope:** compact upright card symbol containing won count and sideways card symbol containing lost count, adjacent and accessible, calculated from viewer partnership.
- **Non-goals:** no scoring change and no history implementation before UX-G1; during GUX it may be a noninteractive element or disabled future entry point.
- **Affected:** status bar trick fact, focused indicator, viewer partnership selector, CSS/SVG using flat project tokens.
- **Architecture:** counts derive from authoritative `tricksNS/tricksEW`; orientation has semantic label beyond shape. Public interaction contract is prepared without fetching/leaking history.
- **Dependencies:** UX-01, UX-06; ENG-01 activates history after gate.
- **Sequence:** count selector → visual primitive → navbar placement → accessible/future-button state.
- **Acceptance:** won/lost counts appear inside unmistakably vertical/horizontal card shapes and remain compact.
- **Browser/mobile:** no clipping at navbar breakpoints; orientation remains distinguishable under zoom/high contrast.
- **Regression tests:** NS/EW viewer mapping, 0–13 counts, no-seat fallback, semantics and snapshots.
- **Completion gate:** UX-11 PASS requires correct viewer-relative counts and visual regression coverage.

### UX-12 — Invite code

- **Problem:** dedicated clipboard button is unnecessary and makes basic sharing depend on Clipboard API.
- **Scope:** show actual invite code as selectable text in relevant dropdown/menu; remove dedicated copy button; preserve invite URL only where the real product flow needs link sharing.
- **Non-goals:** no required Clipboard API and no exposure outside authorized table participant UI.
- **Affected:** waiting invite block, active table menu, local copied/share state removal where obsolete.
- **Architecture:** semantic selectable text is the baseline workflow; URL generation remains isolated only if another explicit share action consumes it.
- **Dependencies:** existing invite projection/session access.
- **Sequence:** locate both invite surfaces → render/selectable code → remove button/state/API → verify privacy.
- **Acceptance:** user can select and manually copy full code; there is no dedicated `copy code` button.
- **Browser/mobile:** long-press/text selection works and code does not truncate irrecoverably at narrow widths.
- **Regression tests:** code visibility/selection CSS, no clipboard call, authorized route only.
- **Completion gate:** UX-12 PASS requires manual-copy behavior on desktop/mobile.

### UX-13 — Bot identity

- **Problem:** bot identity currently appears as secondary text and placement varies with seat layout.
- **Scope:** compact robot icon immediately to the right of participant name in all horizontal/vertical positions and portal identity where applicable.
- **Non-goals:** no oversized badge, explanatory copy, bot strategy, or consensus behavior change.
- **Affected:** participant identity/name row, bot icon primitive or inline SVG, portal/header, tests.
- **Architecture:** one identity composition owns name + owner marker + bot marker consistently; icon has accessible bot semantics without repeated noisy announcements.
- **Dependencies:** UX-01 participant extraction; projected `isBot` already exists.
- **Sequence:** identity component → right-side placement → vertical seats → portal/accessibility.
- **Acceptance:** every bot name has a clearly visible right-side robot icon; human names do not; placement is consistent.
- **Browser/mobile:** icon stays aligned with truncated/wrapped names at all seat orientations and widths.
- **Regression tests:** human/bot/owner+bot identity, long names, four positions, accessible name.
- **Completion gate:** UX-13 PASS requires component and screenshot evidence for all positions.

### UX-14 — Cross-breakpoint gameplay verification

- **Problem:** current compile/unit checks and one desktop E2E cannot detect gameplay overlap, latency feedback, touch, animation order, or orientation regressions.
- **Scope:** automated browser fixtures/scenarios and screenshot/bounding-box evidence for auction, opening lead, dummy reveal/top/left/right, declarer own/dummy, defender, complete/next trick, desktop/touch drag, optimistic bid/play, rejection rollback, reconnect with optimistic action, board-click animation skip, claim, undo, bot identity, contract ownership, navbar actions, and trick indicator.
- **Non-goals:** manual eyeballing alone is insufficient; no production-only fixture backdoor; no broad E2E duplication of cheap reducer tests.
- **Affected:** Playwright config/specs, deterministic test fixtures/API where already sanctioned, component/unit tests, CI artifacts.
- **Architecture:** cheapest-level rule: reducers/queues/selectors in unit tests, protocol/reconnect in integration tests, and only spatial/real interaction wiring in browser tests. Screenshot baselines are viewport- and state-named.
- **Dependencies:** UX-01 through UX-13 complete; existing four-context Playwright flow and hidden-frame assertion preserved.
- **Sequence:** stable deterministic scenarios → desktop behavior → tablet/mobile/touch → delayed/rejected realtime → screenshots/geometry → accessibility/reduced-motion → CI shard/runtime review.
- **Acceptance:** every required scenario has recorded evidence; visual checks detect overlap, clipping, unreadable rank/suit, misplaced trick slots, and inconsistent scale at wide desktop, laptop, tablet landscape/portrait, common mobile, and 320–400 px.
- **Browser/mobile:** explicit project matrix includes Desktop Chrome and a mobile touch profile at minimum; manual real-device sanity is recorded for drag/audio where emulation is insufficient.
- **Regression tests:** retain unit/component, interaction, realtime, and visual suites enumerated in section 21; raw network privacy assertions remain active.
- **Completion gate:** UX-14 PASS requires green automated matrix, approved screenshots, no severity-high UX/accessibility regression, and evidence links recorded in section 23.

## 20. GATE UX-G1 — Gameplay UX Refactor Complete

Status awal: **BLOCKED — seluruh UX objective masih NOT STARTED**.

```text
[ ] UX-01 PASS  [ ] UX-02 PASS  [ ] UX-03 PASS  [ ] UX-04 PASS
[ ] UX-05 PASS  [ ] UX-06 PASS  [ ] UX-07 PASS  [ ] UX-08 PASS
[ ] UX-09 PASS  [ ] UX-10 PASS  [ ] UX-11 PASS  [ ] UX-12 PASS
[ ] UX-13 PASS  [ ] UX-14 PASS
```

Gate hanya PASS bila seluruh fourteen objectives memiliki acceptance, browser/mobile, regression, dan completion evidence. TypeScript compile, satu screenshot, atau happy path desktop tidak cukup. **ENG-01, ENG-02, dan ENG-03 dilarang mulai sebelum UX-G1 PASS.** Jika frontend menemukan engine limitation, catat contract/dependency dan lanjutkan frontend dengan boundary/fixture yang aman. Exception engine sebelum gate hanya boleh dibuat bila frontend contract literal mustahil diselesaikan tanpanya, disertai blocker reproduction, smallest compatible change, ADR amendment, dan explicit approval di roadmap.

## 21. Required implementation order and test strategy

### Stage A — Audit and regression baseline

- Preserve/complete current Playwright four-context flow, capture desktop/mobile states, inventory authoritative fields/events/ACK ordering, and record current collision/latency failures.
- Establish deterministic delay/reject/resync test controls without production exposure.

### Stage B — Behavior-preserving extraction

- Complete UX-01 alone. Extraction commits must not smuggle geometry, motion, copy, or optimistic behavior changes.

### Stage C — Interaction state

- Implement UX-02, then UX-03. Lock authoritative/optimistic/presentation boundaries before visual motion.

### Stage D — Canonical gameplay presentation

- Implement UX-06, UX-07, UX-08, UX-09, UX-11, UX-12, and UX-13 in focused changes.

### Stage E — Motion and physical interaction

- Implement UX-04, UX-05, and UX-10 after stable layout anchors and canonical action paths exist.

### Stage F — Verification

- Complete UX-14, record evidence, and evaluate UX-G1. Do not waive a failed orientation/mobile/reconciliation scenario as polish.

### Stage G — Engine extensions

- Only after UX-G1: ENG-01 Play History → ENG-02 Table Score Sheet → ENG-03 Bot Consensus Behavior. ENG-03 may precede ENG-02 only if a reproduced consensus softlock blocks score-sheet/session verification; record that dependency change first.

### Required test layers

- **Unit/component:** canonical card variants/rank/suit/playable state; bot marker; contract formatter; viewer-relative trick indicator; optimistic reducer/rebase; animation queue/skip; audio turn detector; capability matrix.
- **Interaction:** auction calls, click/tap/drag play, touch pointer cancellation, claim, undo, participant portal, board-only animation skip.
- **Realtime/integration:** delayed ACK/event in both orders, rejection, revision conflict/out-of-order event, unrelated event rebase, reconnect/snapshot, remote action during animation, hidden-projection assertions.
- **Visual/browser:** auction, play, dummy top/left/right, completed trick/winner, next trick, desktop/tablet/mobile. Assert bounding boxes as well as screenshots.

## 22. Post-gate engine objectives

### ENG-01 — Play history

- **Problem:** completed-trick review has no governed UI entry point or explicit viewer information policy.
- **Scope:** activate UX-11 indicator; Dummy viewer may see trick 1 through latest available trick, non-Dummy sees only last trick allowed by approved policy; persist/hydrate history in authoritative state and project only entitled records; sheet/popover UI.
- **Non-goals:** no hidden-hand leak, full replay/editor, public spectator history, or scoring change.
- **Affected:** pure engine history state only if current `CompletedTricks` is insufficient, table projector, WS/schema/snapshot, persistence compatibility, web history overlay.
- **Architecture:** server projection enforces entitlement; frontend never filters a richer unauthorized payload as the security boundary. Existing public `CompletedTricks` exposure must be privacy-reviewed and may require narrowing/versioning.
- **Dependencies:** UX-G1 PASS, UX-11 entry point, hidden-information policy decision, snapshot compatibility.
- **Sequence:** policy tests → state necessity audit → projector/schema → reconnect → UI → privacy network inspection.
- **Acceptance:** Dummy sees allowed range; others see only allowed last trick; refresh/reconnect is consistent; unauthorized tricks absent from raw payload.
- **Browser/mobile:** accessible compact sheet/popover scrolls and closes correctly without moving table geometry.
- **Regression tests:** viewer matrix for declarer/dummy/defenders, trick 0–13, reconnect, projection raw frames, legacy snapshot.
- **Completion gate:** ENG-01 PASS requires privacy review and network evidence, not only hidden DOM.

### ENG-02 — Table score sheet

- **Problem:** there is no durable current-table/session score sheet, and “IMP pair-oriented” semantics are ambiguous relative to existing single-board duplicate score and future two-table Team Match.
- **Scope:** define exact product semantics first; source board results from durable table lifecycle, aggregate/convert to IMP only against a documented comparison/reference, preserve pair identities across seats/boards, expose navbar sheet from table start through current board, and survive reconnect.
- **Non-goals:** do not silently call raw duplicate points IMP, do not implement rubber scoring/tournament movement, and do not rewrite pure scoring without a proven domain gap.
- **Affected:** product contract/ADR, board result persistence/query, application score aggregation, API/WS projection as appropriate, navbar score sheet.
- **Architecture:** canonical `score_ns` remains source result; IMP conversion uses one documented comparison and orientation. Pair/session lifecycle and identity must be explicit before schema work.
- **Dependencies:** UX-G1 PASS; domain decision on comparison baseline, pair identity, table/session boundary, and relationship to Phase 5 Team Match.
- **Sequence:** terminology/domain workshop → fixtures/acceptance examples → persistence/query design → service/projection → UI → reconnect/history tests.
- **Acceptance:** every row traces to durable board result; aggregation/sign/IMP fixtures match approved semantics; pair identity is stable; refresh/restart retains same sheet.
- **Browser/mobile:** navbar action and sheet remain readable/scrollable at narrow widths with concise headers.
- **Regression tests:** passed out, positive/negative/tie, seat orientation, multiple boards, duplicate delivery, restart/reconnect, authorization.
- **Completion gate:** ENG-02 remains BLOCKED until semantics are accepted in product contract/ADR; UI mock alone cannot pass it.

### ENG-03 — Bot consensus behavior

- **Problem:** current ADR/code disables claim and undo whenever any bot is seated; the requested partner-follow policy instead needs deterministic server responses and termination without softlock.
- **Scope:** server/aggregate/actor owns bot vote decisions. If bot partner is bot, bot side rejects request per policy. If bot partner is human, bot follows that human partner's accepted/rejected response. Define initiator partnership, response order, reconnect, and terminal transitions.
- **Non-goals:** frontend never fabricates bot votes; no strategic claim evaluation, autonomous proposal, or bot AI redesign.
- **Affected:** ADR-007/ADR-008 at implementation time, aggregate action request model, bot automation/actor, projector/events/protocol if bot response needs representation, frontend consensus progress.
- **Architecture:** one durable human response may deterministically schedule bot follow-up through the same actor/repository path; bot-bot side deterministically rejects. Exactly one terminal outcome and no unresolved request after all possible decisions.
- **Dependencies:** UX-G1 PASS and explicit supersession of current “consensus unavailable with any bot” decision.
- **Sequence:** state-transition table → aggregate tests → actor automated response → persistence/reconnect → projection/frontend → softlock E2E.
- **Acceptance:** bot+bot partnership rejects and terminates; human+bot partnership follows human for accept/reject; initiator edge cases deterministic; restart/reconnect reaches same result; no pending request can become unanswerable.
- **Browser/mobile:** consensus progress/result updates without fake local bot action and controls recover after terminal decision.
- **Regression tests:** claim/undo × bot-bot/human-bot × accept/reject × requester partnership × response order × duplicate/restart/reconnect.
- **Completion gate:** ENG-03 PASS requires exhaustive transition tests and no unresolved action request; current global bot-disable behavior must be intentionally superseded, not silently bypassed.

## 23. Evidence ledger and unresolved risks

Record implementation evidence here; all entries begin empty:

```text
UX-01 extraction/tests: ____________________
UX-02 reconciliation traces: ______________
UX-03 no-invalid-frame proof: ______________
UX-04 motion/skip recordings: ______________
UX-05 desktop/mobile drag: _________________
UX-06 viewport screenshots/geometry: _______
UX-07–13 component/browser evidence: _______
UX-14 matrix report: _______________________
UX-G1 review/date: _________________________
ENG-01 privacy review: _____________________
ENG-02 score semantics decision: ___________
ENG-03 consensus transition report: ________
```

Known risks discovered in the current repository:

1. ACK is sent before projected broadcast, but client ignores ACK and events lack origin request ID. Reconciliation design must prove safe correlation or introduce the smallest compatible protocol metadata change.
2. Every event/snapshot currently clears every pending command. Optimistic work requires per-operation settlement/rebase and must not restore the unsafe behavior of arbitrary offline retry.
3. Global `hasPendingCommand` serializes all UI mutation and can hide race problems during optimistic work; replace only with explicit capability/conflict rules.
4. `CompletedTricks` is already projected to all participants. ENG-01's intended “Dummy all / others last allowed” policy conflicts with that payload and needs a security/product decision before UI.
5. ENG-02 terminology is unresolved: a single table has no inherent IMP comparison. Raw duplicate score cannot be labeled IMP without a reference result or accepted pair/session model.
6. ADR-008 and current aggregate explicitly disable consensus whenever a bot is present. ENG-03 is a deliberate future product/architecture supersession and remains gated.
7. Playwright/config/e2e files are currently uncommitted alongside unrelated backend/frontend changes. Baseline ownership must be resolved before implementation commits so tests are not accidentally overwritten.
8. CSS has many orientation-specific offsets but no shared board-zone contract; motion built before UX-06 would couple animation to fragile magic numbers.
