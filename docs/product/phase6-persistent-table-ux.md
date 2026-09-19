# Phase 6 — Persistent Table & Human-Centered Application UX

Status: **PLANNED — belum diimplementasikan**. Audit: 19 September 2026, baseline `9738c90`.
Pemilik: product/engineering. Roadmap induk: [PLAN.md](../../PLAN.md); baseline frontend: [apps/web/PLAN.md](../../apps/web/PLAN.md).

## Objective

**The table is a persistent session, not a page.** Pemain dapat membuka Friends, Settings, History, atau informasi match dan kembali bermain tanpa menutup subscription meja, mengambil ulang seluruh state yang masih valid, atau melihat splash pemulihan baru. State terus menerima perubahan authoritative selama workspace lain dibuka. Navigasi tidak melepaskan seat, mengganti board, atau menjalankan aksi permainan.

Phase 6 memperluas shell dan komponen yang ada, memperjelas informasi bridge, dan memperbaiki aksesibilitas lintas perangkat. Tidak ada rewrite engine, protokol realtime baru, library state baru, atau design system kedua.

“Sama ketika kembali” berarti board, hand, auction, trick, dan score tidak berubah **akibat navigasi**. Jika pemain lain melakukan aksi, undo disetujui, atau server memajukan board secara sah, tampilan kembali menunjukkan state terbaru; permainan tidak dibekukan pada snapshot sebelum membuka workspace.

### Batas phase sebelumnya

- GUX UX-01–14, ENG-01–03, replay, DDS, account/Friends/chat, dan implementasi Team Match telah memiliki kode dan evidence; jangan membuatnya ulang.
- Header roadmap masih menyatakan Phase 3 berjalan, Phase 4 menunggu review independen, dan Phase 5 menunggu release gates. Checkpoint 16 September menyatakan hardening lokal PASS, **bukan** seluruh Phase 0–5/release sudah selesai. Phase 6 tidak menandai gate lama selesai.
- Supported single-instance hosting, independent WBF review, dan pilot delapan manusia tetap gate terpisah. Persistent frontend tidak menyelesaikan actor/broadcast lintas instance di Vercel.
- Tidak termasuk ranking, matchmaking, public profile, global casual-game archive lintas device, matchpoints, timer pertandingan baru, AI bot, atau perubahan aturan bridge.

## Current-State Audit

Path di tabel relatif terhadap `apps/web/app/`, kecuali dinyatakan lain. Temuan runtime dan temuan source dibedakan di [evidence audit](phase6-audit.md).

| Area | Implementasi aktual / gap | Keputusan Phase 6 |
|---|---|---|
| Root dan routing | `layout.tsx` hanya root HTML; `(app)/layout.tsx` membungkus Play/Friends/Settings/Match. `lobby/layout.tsx` me-re-export layout itu, tetapi bukan ancestor bersama. `table/[tableId]/page.tsx` berada di luar keduanya. | Satu shared layout route group untuk seluruh authenticated workspace, lobby, dan table; URL publik tetap. |
| Session ownership | `bridge-table.tsx` memanggil `useTableSession()`. `lobby/lobby-client.tsx` membuat instance lain dengan `connectOnRestore: false`. Cleanup hook memanggil `stopConnection`; restore membaca akun, marker, dan GET table. | Panggil hook owner satu kali dari persistent provider. BridgeTable dan LobbyClient menjadi consumer. |
| Realtime akun | `account-presence.tsx` membuka socket sendiri kecuali `compact`; table memakai mode compact karena hook table membawa chat/social. Kedua jalur memiliki retry dan rotation 285.000 ms. | Satu connection owner per authenticated app instance; heartbeat dan notice UI tetap ada, tanpa socket kedua. |
| Chat | `chat-store.ts` adalah external store yang menerima satu socket lewat `attach`, dengan target/open tracking, optimistic delivery dan dedup. | Pertahankan API store dan protocol. Jangan membiarkan dua owner bergantian mengambil pointer socket. |
| Reconciliation | `table-state.ts`, `optimistic-gameplay.ts`, `table-projection.ts`, `gameplay-capabilities.ts` sudah memisahkan authoritative projection, pending operations, normalisasi, dan capability. | Pertahankan reducer/request ID/revision/seq/controller fence. Navigation bukan action reducer game. |
| Gameplay | `BridgeTable` mengorkestrasi domain components; `PlayingCard`/`BridgeHand`, `TableSurface`, `CurrentTrick`, participant, waiting room, consensus sudah terpisah. | Pindahkan ownership, bukan tulis ulang renderer. Tidak perlu memecah setiap JSX kecil. |
| Global effects | Shortcut auction memakai `window.keydown`; motion, audio dan BoardResult mempunyai effects sendiri. Table yang dipertahankan mounted dapat tetap menangkap input workspace. | Tambahkan batas visibility/focus/input yang eksplisit; background game reconciliation tetap hidup. |
| Result | `table/board-result.tsx` menutup result setelah 5 detik atau klik dokumen, lalu dapat mengirim `table.next_board`. `BridgeTable` tidak memakai opsi `persistent`. | Ganti auto-advance presentasional dengan action Next board eksplisit. Ini perlu agar membuka workspace/klik Friends tidak mengubah board. |
| Navigation | `app-navigation.tsx` punya Play/Friends/History/Deals/Settings. History dan Deals hanya memunculkan Coming soon. Table tidak memiliki navigation rail/bottom nav ini. | Table menjadi destination utama saat aktif; History menjadi fitur nyata terbatas sumber yang tersedia; hapus placeholder dari primary nav. |
| Settings/Profile | `/settings` menampilkan Profile; `profile-settings.tsx` menyimpan nama/avatar dan punya logout sendiri. Save memanggil `router.refresh()`. | Satu workspace Settings dengan bagian Profile, bukan dua formulir/route duplikat. Refresh profile tidak remount session. Satukan jalur logout. |
| Trick history | `table/trick-indicator.tsx` memakai popover, posisi relatif seat dan canonical card, prev/next. Belum menampilkan leader/winner/order secara eksplisit. Pemilihan reset ke latest ketika trigger diklik. | Tambah metadata compact dan aturan selection/scroll. Jangan mengubah entitlement server. |
| Auction | `AuctionTable` sudah empat kolom W/N/E/S, Pass/X/XX dan dealer-aligned rows; selalu auto-scroll saat call bertambah. | Pertahankan konvensi; auto-follow hanya ketika pengguna sudah di bagian terbaru. Tambah dealer/turn non-color cue. |
| Score/replay | `ScoreSheet` membaca projected `scoreSheet`, NS/EW dan replay; `BoardReplayModal` dan DDS sudah tersedia. Ringkasan result hanya Contract/Result/Score. | Tambah declarer/vulnerability/perspektif score dan mobile board rows; reuse replay/analysis boundary. |
| Match | `team-match.tsx` memiliki setup, daftar milik participant, polling detail, readiness, progress dan final IMP. Kembali ke match mempertahankan seat server tetapi melepas connection table page. | Jadikan match workspace sekunder; tetap gunakan room milik viewer. Tangani refresh entitlement match selesai tanpa bergantung remount table. |
| UI primitives | Native button/input/details/dialog/popover, focus CSS, drag dialog, `IssueNotice`, token `globals.css`. `package.json` tidak memasang React Aria/TanStack Query meski disebut baseline plan lama. | Reuse native primitives dan fetch/reducer existing. Tidak menambahkan kedua library itu demi menyamakan dokumen lama. |
| Visual/accessibility | Browser auction 1440/1024/768/390/320 tidak horizontal overflow. Di 320, navbar dua baris, nama panjang terpotong, hand overlap rapat; beberapa kontrol di bawah 44 px. Friends memakai eyebrow/heading/copy; bottom nav lima item dengan label `.65rem`. | Pertahankan felt, kartu dan palette; rekomposisi ruang dan target, kurangi chrome/copy. Tidak menyatakan layout existing sepenuhnya gagal. |
| Timer | Tidak ditemukan game clock/deadline pemain dalam `GameProjection`; timer yang ada termasuk motion, result, retry, heartbeat/rotation. | Jangan mengarang clock baru. Pertahankan lifetime timer session, pisahkan timer presentation; deadline authoritative masa depan memakai server time. |

### Gap dokumen terhadap kode

1. Scope/non-goal awal menyebut guest-only, tanpa Friends/chat; ADR 0018 dan 0019 serta kode telah menggantikannya. Audit mengikuti account/Friends/chat existing.
2. “Persistent BridgeTableShell” di plan web sebelumnya menjaga kontinuitas fase permainan; belum memenuhi kontinuitas antar-workspace. Phase 6 memperluas lifetime boundary ini.
3. ENG-01/02 selesai berarti data history/scoring tersedia, bukan History global di navbar sudah berfungsi. Tidak ada endpoint daftar seluruh casual table milik akun dalam contract yang diaudit.
4. IMP bukan sekadar future extension: Phase 5 sudah mengimplementasikannya untuk Team Match. Matchpoints/rankings tetap di luar scope.
5. [ADR 0013](../adr/0013-recipient-scoped-play-history.md): Dummy mendapat seluruh completed tricks; declarer/defender/unseated hanya latest. Jangan merekonstruksi trick lama dari frame yang pernah dilihat.
6. [ADR 0020](../adr/0020-internal-team-match.md): replay/DDS dan detailed cross-room results dibatasi sampai match complete. Cache/presentation tidak boleh melewati batas itu.

## UX Principles

- Navigasi mengubah workspace; hanya tindakan lifecycle eksplisit mengubah membership/active table.
- Prioritas: kartu/current trick → active seat → dummy/own hand → contract/declarer/vulnerability/dealer → auction → trick history → score → informasi lain.
- Table adalah default visual surface; hanya satu workspace sekunder aktif. Tidak menumpuk sidebar, chat, history, dan modal sekaligus di ruang kartu.
- Gunakan heading/action domain yang singkat. Hapus subtitle yang mengulang heading, nomor dekoratif, dan Coming soon destination. Informasi konsekuensi leave/logout dan batas akses tetap boleh terlihat.
- Compact tidak berarti font atau hit area kecil. Recompose dan progressive disclosure mendahului pengecilan kartu.
- Focus, label, state non-color, dan reduced motion adalah requirement awal.
- Motion menjelaskan transisi; legal action dan reconciliation tidak menunggu animasi.
- `globals.css` sumber token; flat color, tanpa gradient, nested cards/dekorasi baru.

## Information Architecture

| Destination | URL / entry | Saat aktif | Tanpa active table | Alasan |
|---|---|---|---|---|
| Table / Play | `/table/[tableId]` / `/play` | Table membuka surface yang sudah hidup, indikator board/turn pada item yang sama | Play membuka pilihan mode existing | Satu anchor utama, tanpa return banner duplikat |
| Friends | `/friends` | Secondary workspace berisi `SocialUsers`, pencarian, follow dan private chat | Halaman normal di shell | Sering diakses; bukan dependency gameplay |
| History | `/history` baru | Board history meja aktif + akses ke match milik akun | Daftar match dari endpoint existing; empty state bila tidak ada | Memenuhi akses history tanpa mengarang global archive |
| Settings | `/settings` | Secondary workspace; Profile dan preferensi existing | Halaman normal | Utility jarang dipakai, tempatkan di More |
| Profile | `/settings#profile` | Fokus ke section Profile existing | Sama | Tidak perlu duplicate destination/form atau public profile |
| Match information | `/match/[matchId]` | Secondary workspace dari navbar table | Detail/setup existing | Contextual, bukan primary nav semua pengguna |
| Auction / trick / board detail | Trigger table; History dapat memilih board | Panel kontekstual; return tetap ke table/history asal | Tersedia hanya jika sumber authorized ada | Akses cepat saat bermain |
| Notifications | Notice/chat/invite existing, tombol More jika ada notice yang dapat dibuka | Buka notice/detail tanpa leave | Sama | Tidak membuat inbox durabel baru untuk event live-only |
| More | Local popover/sheet | Settings, Profile, Help, sound, logout; action table di tempat kontekstual | Settings/Profile/Help/logout dan akses Play | Maksimal empat primary mobile destinations |
| Help | Panel lokal More | Shortcut/cara kontrol dan batas fitur yang relevan, tanpa marketing copy | Sama | Tidak memerlukan backend/route baru |
| Rankings / Deals placeholder | Tidak ditampilkan | Tidak tersedia | Tidak tersedia | Belum ada capability/backend; tidak menjanjikan fitur palsu |

History Phase 6 secara eksplisit **bukan arsip semua casual table**. Gunakan score sheet session yang terotorisasi dan daftar match participant existing (bounded 20 entries, unfinished-first). Tampilkan batas list bila relevan; jangan mengklaim pagination/global completeness yang endpoint belum sediakan. Setelah explicit leave, bersihkan data restricted meja; archive lintas sesi memerlukan product/API work terpisah.

## Desktop Behaviour

- Rail compact di kiri: Table/Play, Friends, History, More; identity/Settings dapat berada di bagian bawah. Icon-only diberi accessible name dan tooltip singkat pada hover maupun focus. Active workspace memakai `aria-current`; status table aktif memakai indikator terpisah, bukan dua item `aria-current`.
- Pada desktop lebar dengan ruang cukup, workspace sekunder menempati kolom kanan dan table tetap menjadi area terbesar. Gunakan satu slot panel untuk Friends/History/Settings/chat; membuka yang lain mengganti isinya.
- Layout memakai **lebar area table yang tersisa**, bukan hanya lebar viewport. Jika panel membuat kartu/target/zone gagal, fallback ke panel overlay yang lebih luas; jangan memaksa side-by-side laptop 1024 px.
- Panel nonmodal tidak menjebak focus; terdapat heading, close, dan jalan keyboard kembali ke table. Jika panel menjadi modal, background inert dan input gameplay dinonaktifkan sampai ditutup.
- Table yang tetap terlihat boleh dimainkan lewat pointer ketika tidak terhalang panel nonmodal. Shortcut huruf hanya aktif ketika focus berada dalam region table; mengetik/mengoperasikan Friends tidak mengirim call.
- Current board/turn cukup berada pada Table item atau status bar existing. Tidak ada mini-table kedua, floating return card, maupun banner penjelasan besar.

## Mobile Behaviour

- Bottom navigation empat item maksimum: **Table/Play, Friends, History, More**, icon + label singkat. Target 44×44 px; bottom safe area diperhitungkan dalam tinggi tersedia table.
- Auction, current trick, dummy dan own hand memakai zona yang direkomposisi. Di hand 13 kartu, gunakan baris suit/reflow ketika exposure target tak memadai; jangan mempertahankan fan sempit dengan font lebih kecil.
- Friends/Settings/History menjadi secondary screen pada slot workspace yang sama. Table tetap mounted di belakang tetapi hidden/inert, bukan dipaksa menjadi miniatur. Bottom nav memberi satu-tap return dan status turn/board compact.
- Auction/trick detail memakai sheet ringan dengan daftar vertikal/pager. Full-height secondary screen hanya untuk isi panjang atau keyboard yang membutuhkan ruang, bukan semua menu berupa fullscreen modal.
- Workspace scroll terpisah dari table. Virtual keyboard tidak menutupi input, close control, atau tombol kirim chat. Pergantian portrait/landscape tidak mereset table/history selection.
- Jika giliran tiba saat workspace terbuka, indikator Table berubah dan satu live announcement berbunyi; audio mengikuti mute existing. Jangan memindah route/focus otomatis atau mengeluarkan toast berulang.

## Persistent Table Architecture

### Pilihan minimal

Perluas shared `(app)/layout.tsx`; pindahkan route files table dan lobby ke route group yang sama tanpa mengubah URL. Persistent client boundary ditempatkan di bawah server auth layout. Tidak menggunakan iframe, duplikasi table per route, dynamic route cache sebagai session store, parallel/intercepting routes, atau framework state baru.

Route workspace normal tetap dipakai untuk deep link/Back/Forward. `AppShell` mempertahankan satu instance table **di luar slot `children` yang berganti**. Route table hanya memvalidasi/aktivasi identity; tidak merender instance `BridgeTable` kedua. Komponen domain di `app/table/` dapat tetap pada lokasi existing; hanya page route yang dipindah.

```text
RootLayout (server: metadata, html, globals)
└─ (app)/layout (server: auth/account boundary)
   └─ GameSessionProvider (client, stabil selama akun sama)
      └─ AppShell (client: tata letak dan presentasi workspace)
         ├─ AppNavigation (existing, rail/bottom adaptation)
         ├─ AccountPresence (heartbeat/notices, consumer transport)
         ├─ persistent table region (key hanya activeTableId)
         │  └─ TableSocialProvider
         │     └─ BridgeTable (consumer session)
         │        └─ existing table/* components
         └─ workspace region: children route yang sedang aktif
            ├─ /table/[tableId]: activation boundary, bukan table copy
            ├─ /play, /lobby
            ├─ /friends, /settings, /history
            └─ /match, /match/[matchId]
```

Server pages/guards dan metadata tetap Server Components. Jangan mengubah semua feature pages menjadi client hanya untuk mempertahankan table. Error/loading workspace dibatasi pada slot workspace; error Friends tidak boleh mengganti ancestor provider atau menghilangkan surface table. Auth failure adalah exception yang memang membersihkan session.

### Ownership dan masa hidup

| State | Owner | Bertahan ketika menu berubah | Reset/invalidation |
|---|---|---|---|
| Active identity, recovery status, tableId, credentials | `GameSessionProvider` melalui refactor `useTableSession` | Ya | Logout, account berubah, access revoked, explicit leave/switch |
| Projection, revision, lastSeenSeq, controller epoch, pending request IDs | Reducer existing di session owner | Ya; terus reconcile | Aturan reducer existing untuk snapshot/conflict/disconnect; tidak reset karena URL |
| Socket, retry/backoff/rotation, subscription, credential refresh single-flight | Satu owner di session hook | Ya | Network/auth/rotation/explicit table lifecycle; bukan workspace |
| Chat message store/optimistic delivery | `chatStore` existing, identity scoped | Ya | Account berubah, permission revocation, retention existing |
| Active workspace dan detail bookmark | URL + navigation state shell | Back/Forward source of truth | Navigasi berikutnya; tidak disimpan dalam reducer game |
| Friends search/filter/scroll; Settings draft/status; history board/trick/scroll | State presentasi di shell per workspace, bounded dan identity scoped | Ya; latest mounted workspace boleh unmount | Account berubah; saved draft; table/board/entitlement invalidation |
| Selected bid/drag/motion; table popover position/scroll | Table presentation boundary yang persistent | State relevan ya; gesture/motion sementara dibatalkan ketika tertutup | Board/seat/capability berubah; tidak mengirim stale action |
| Replay/analysis data | Existing loader + bounded session cache bila diperlukan | Selected authorized board/step tetap | Permission/match/table berubah; abort stale request; bukan localStorage |

Tidak membuat global persisted state untuk seluruh UI. Simpan hanya input/filter/selection/scroll yang diperlukan; buang workspace resource yang sudah invalid. Background REST requests boleh abort ketika workspace ditutup, tanpa abort connection/table reconciliation. Response lama harus memeriksa identity/table generation sebelum diterapkan.

### Lifecycle dan navigation contract

| Kejadian | Behaviour wajib |
|---|---|
| No Active Table | Shell normal; satu account realtime connection untuk chat/social, tanpa table subscription. Table item berlabel Play. |
| Create/join pertama | Gunakan lifecycle API existing; setelah sukses set active table dan subscribe satu kali lewat connection owner. Lobby tidak membangun hook kedua. |
| Cold direct link / refresh | Server auth guard, restore identity dan marker tervalidasi; satu bootstrap GET/snapshot/reconnect diperbolehkan. Loading terbatas table region; jangan menganggap localStorage sebagai game authority. |
| Refresh pada `/settings` saat ada marker table | Pulihkan table di belakang sekali, tetap tampilkan Settings. Tidak memaksa redirect ke table/lobby. |
| Table → Friends → Settings → History → Table | Soft navigation URL; mount session tetap, socket dan subscription sama, tidak ada leave/GET full table/ticket baru akibat navigasi. Restore scroll/selection bila masih valid. |
| Back/Forward antar workspace | Mengubah workspace sesuai URL tanpa join/leave/reset. Tidak menambahkan history entry untuk tiap snapshot, animation frame, atau update search keystroke. |
| Close workspace | Jika dibuka dari Table dan previous entry adalah table yang sama, Back. Jika direct link atau sebelumnya workspace lain, `replace` ke Table (atau Play tanpa table); tidak mengirim user keluar situs dengan blind Back. |
| Buka target table lain / invite | Tampilkan pilihan tetap di table sekarang atau pindah secara eksplisit. Jangan memanggil `openTable` hanya karena URL berbeda. Jika lifecycle lama melarang leave/switch (fixed Team Match), gunakan capability/flow existing dan jelaskan singkat. |
| Switch disetujui | Validasi akses tujuan sejauh API existing mengizinkan; leave meja lama harus sukses dahulu bila diwajibkan, bersihkan subscription/pending/presentation, lalu join/open tujuan. Jika tujuan gagal sesudah leave, tampilkan recovery lobby; jangan mengaku perpindahan atomic. |
| Explicit leave | Confirm konsekuensi existing; server success dahulu, lalu clear marker/table/cache scoped. Failure mempertahankan table dan menyediakan Retry. Match information bukan Leave. |
| Board selesai | Result tetap tersedia; owner/capability yang sah memilih Next board/Finish. Timeout visual, klik workspace, dan unhide tidak mengirim next-board. |
| Table FINISHED | Tampilkan result/history authorized hingga explicit dismiss; tidak menandai masih bermain. Jangan menghapus record semata-mata karena board scored. |
| Logout / account invalid / removal / expiry | Satu cleanup path menutup socket, membatalkan requests, menghapus pending/cache/draft sensitif dan marker sesuai sebab. Hanya successful logout/revocation memutus session; logout infrastructure error tidak pura-pura sukses. |
| Multi-tab / takeover | Pertahankan controller fencing existing; workspace nav tidak mengambil alih controller. Storage clear lintas tab tetap menghentikan recovery. Jangan menyatukan socket lintas tab dengan SharedWorker/BroadcastChannel baru. |

Back dari entry aplikasi menuju situs lain atau full browser refresh memang dapat menutup document/connection. Jaminan tanpa reconnect berlaku pada navigasi internal selama document hidup. Browser sleep/OS suspend ditangani resume, bukan dijanjikan socket abadi.

### Background presentation contract

- Reconciliation, presence dan pending ACK/event tetap berjalan ketika table tertutup. Kembali menampilkan latest projection tanpa memainkan ulang antrean animasi lama.
- Batalkan drag/pointer capture saat workspace modal terbuka; klik penutup, gesture sheet dan keydown workspace tidak dapat diteruskan ke kartu.
- Tutup native top-layer popover/dialog table sebelum table hidden; `hidden` parent saja tidak cukup menjadi kontrak dismissal. Simpan selection/scroll yang relevan, bukan dialog destructive yang belum dikonfirmasi. Tidak membuka kembali claim/leave dialog otomatis.
- Reader history yang sedang di trick/row lama tidak dipaksa lompat oleh update baru. Tampilkan kontrol singkat “Terbaru”; clamp selection ketika undo menghapus trick/score row. Ganti board mereset hanya state board lama.
- Live announcement tidak mengulang seluruh hand/auction setiap render. Turn audio dimiliki satu consumer session, dedup dengan transisi existing, menghormati mute dan browser audio permission.
- General game timer belum tersedia. Retry/rotation tetap wall-clock session; timer motion boleh diselesaikan/suspend tanpa memengaruhi legal state. Tidak mengubah consensus expiry di server.

## Gameplay Improvements

| Bagian | Perubahan terukur | Dipertahankan |
|---|---|---|
| Table | Reserve rail/bottom nav dan workspace slot sebelum menghitung zona; seat, dummy, trick, hand tak saling menutupi. Long name tetap punya accessible full name/detail. | Orientation viewer, felt, table/player zones, capability server |
| Cards | Rank/suit tetap terbaca pada hand/dummy/trick; nonplayable tetap kontras, beda state lewat outline/position/affordance selain warna/opacity. Touch exposure cukup atau hand direflow. | Satu `PlayingCard`, shared scale tokens, click/tap/drag satu command path |
| Auction | W/N/E/S tetap; dealer diberi D+accessible name, turn non-color cue; Pass/X/XX dan contract/declarer ringkas; reader scroll tidak ditarik ke bawah. | `auctionRows`, server legalCalls, `BiddingBox` |
| Current trick | Empat seat berorientasi konsisten; leader/order 1–4 dan winner non-color tersedia secara compact atau detail; jangan hitung winner di React. | `CurrentTrick`, projected winner, motion boundary |
| Trick history | Desktop cross-seat view; mobile satu trick per panel/pager dengan “Trick 7 · W menang”, leader dan play order. Current trick diberi “Berjalan”, winner hanya setelah authoritative result. | `completedTrickCount` untuk numbering, `completedTricks` hanya entitlements |
| Board history | Board rows mobile vertikal: nomor, contract/declarer, vulnerability, result, signed score/perspektif; action replay jelas. Tidak mengubah desktop table menjadi horizontal scroll panjang. | `ScoreSheet` source dan immutable boardId; reuse replay component |
| Score | Chain Contract → Declarer → Vul → Result → Score dapat dibaca tanpa prose. Contoh `4♠ · N · NS vul · +1 · NS +650` hanya jika sesuai projected result. Passed out: label+0, tanpa declarer palsu. | `scoreNS`, immutable lineup/pair attribution; tanpa client scoring engine |
| Match score | Raw duplicate points dan IMP diberi unit/perspektif jelas; tampilkan signed Team A IMP dan opposing total sesuai data existing. Pending paired result tidak ditampilkan sebagai nol final. | Phase 5 comparisons/total dan privacy gate |
| Status | Active seat outline/marker, D/Vul/contract compact, satu connection status; screen reader mendapat seat+phase+action yang dibutuhkan. | Existing participant/presence/turn cue |

Live history tidak menjadi replay bebas. Dummy boleh pager seluruh authorized tricks; non-Dummy hanya latest completed trick + current trick. Prev disabled/absent jika tidak berhak; jangan menulis “belum ada” ketika datanya memang restricted. Full completed-board replay memakai endpoint terpisah dan guard existing.

Extension score yang diperlukan sekarang cukup discriminant mode di presentation (`duplicate` atau `team IMP`) menggunakan sumber existing. Matchpoint tidak diberi nilai placeholder, API, perhitungan, atau switch UI sampai engine/product menyediakan sumbernya.

## Navigation Improvements

- **Table:** selalu dapat dicapai dari shell; menampilkan state active/turn dan board compact. Return bukan join, takeover, refresh atau leave.
- **Friends:** reuse `SocialUsers`, pertahankan search/filter/list position dan chat draft ketika menu berubah. Follow/invite tetap API-authorized; menerima invite tidak otomatis meninggalkan table aktif.
- **History:** implementasikan `/history` yang mengomposisi board ledger active table dan match list existing. Tidak menyalin reducer game atau menciptakan casual archive dari localStorage.
- **Settings/Profile:** satu workspace, heading Settings dan section Profile tanpa eyebrow duplikat. Nama/avatar save tidak reset hand/socket. Mute existing tetap preference yang sama di navbar dan Settings.
- **Notifications:** pertahankan ephemeral follow/friend/invite notices dan chat recovery; satu notice area, no self-notice/duplicate. Tidak ada janji unread archive lintas reload. Aksi Open Chat/View User membuka workspace; Join melewati switch guard.
- **More:** utility/help/logout, tidak menampung gameplay utama; desktop tooltip hanya untuk icon tanpa visible label. Gunakan inline SVG existing style bila perlu, bukan package icon baru untuk empat tombol.
- **Copy:** hapus copy berulang di Friends/lobby dan heading oversized workspace; pertahankan penjelasan mutual-follow hanya pada empty/onboarding state yang membutuhkannya. Jangan memperpanjang translation architecture sebagai scope baru; gunakan pola bahasa existing konsisten dan nama bridge yang dikenal.

## Accessibility

- Primary touch controls minimum **44×44 CSS px** (nav, bid, close, history pager, leave/confirm). Icon visual boleh lebih kecil. Card fan tidak boleh mengandalkan exposed strip sempit sebagai satu-satunya cara memilih: reflow ke target yang dapat disentuh.
- Body/form informasi penting minimal 16 CSS px pada default text size; metadata sekunder minimal 14 px. Rank/suit live minimal 18 px sebagai baseline verifikasi, lalu periksa keterbacaan aktual; jangan override user font/zoom. Bukan alasan memperkecil 17.92 px mobile existing lagi.
- Normal essential text contrast ≥4.5:1; large text ≥3:1; meaningful control boundary/focus/state ≥3:1 terhadap background. Ukur token dalam kombinasi aktual, termasuk suit diamond/disabled cards di felt, bukan hanya warna root.
- Semua icon-only control punya accessible name; tooltip hover/focus dismissible dengan Escape, tidak menggantikan nama, tidak muncul pada control berlabel jelas. Disabled icon yang butuh penjelasan memakai teks kontekstual yang dapat diakses, bukan hover-only tooltip.
- Keyboard: Tab/Shift+Tab, Enter/Space, Escape; modal trap dan focus return; nonmodal panel tidak trap. Native button/link, bukan clickable row/div sebagai satu-satunya aksi.
- Active seat/vulnerability/selected/winner tidak bergantung warna saja. Nama panjang terpotong visual tetap utuh dalam accessible name dan detail yang dapat dibuka dengan keyboard/tap.
- Hidden table tidak menjadi focus target atau sumber duplicate landmarks/live announcements. Table dan workspace menggunakan region/landmark yang bernama; jangan membiarkan dua `<main>` aktif ambigu.
- Zoom browser 200% dan reflow setara 400%/320 CSS px dapat mengakses seluruh control; enlargement teks 200% tidak memotong kartu atau footer action. Sheet yang panjang boleh scroll vertikal; page-wide horizontal scroll bukan solusi history.
- `prefers-reduced-motion` mematikan movement nonessential, tetap mempertahankan winner/turn feedback. Audio bukan satu-satunya cue.

## Responsive Behaviour

Lebar berikut adalah **test matrix**, bukan instruksi menambah breakpoint sembarang. Reuse breakpoint existing sekitar 48rem/767 px/1280 px dan sesuaikan menurut ruang konten setelah shell.

| Kelas | Viewport uji | Behaviour / bukti |
|---|---|---|
| Wide desktop | 1920×1080, 1440×900 | Rail; table+secondary column jika zona lolos. Screenshot auction/play/dense dummy/current trick, history panjang. |
| Laptop | 1024×768 | Rail tetap compact; overlay workspace bila berdampingan merusak target/card. Status tidak mengambil mayoritas tinggi. |
| Tablet | 768×1024, 1024×768 touch | Sheet/docked panel menurut ruang; tidak ada hover dependency; mouse dan touch sama-sama legal. |
| Mobile | 390×844, 360×800, 320×568 | Empat bottom destinations, reflow hand/status, readable history tanpa horizontal table scroll, safe-area dan keyboard. |
| Short landscape | 568×320 | Vertical overflow terarah diperbolehkan; close/navigation/kartu playable tetap dapat dijangkau, bukan clipped oleh fixed height. |
| Zoom/text | Desktop 200% dan 400%; teks 200% | Reflow efektif, no overlap, focus tidak tertutup; uji browser zoom nyata selain resize viewport. |

No global overflow saja tidak membuktikan kualitas: periksa bounding boxes antara dummy/trick/players/own hand, actual hit target, rank/suit exposed, close affordance, keyboard, dan pointer drag/tap.

## Component Refactor

| Tindakan | File/component aktual atau usulan | Scope |
|---|---|---|
| Pertahankan | `table-state.ts`, `optimistic-gameplay.ts`, `table-projection.ts`, `gameplay-capabilities.ts` | Domain/state semantics existing; hanya test atau perubahan yang dibuktikan perlu |
| Pertahankan/extend | `table/playing-card.tsx`, `table-surface.tsx`, `current-trick.tsx`, `participant-position.tsx`, `auction-controls.tsx`, `use-card-drag.ts`, `use-gameplay-motion.ts` | Readability/geometry, input ownership, background motion, bukan renderer baru |
| Pindahkan route | `table/[tableId]/page.tsx` → `(app)/table/[tableId]/page.tsx`; `lobby/page.tsx` → `(app)/lobby/page.tsx` | URL tetap; hapus redundant lobby layout wrapper; content modules tidak wajib pindah |
| Buat meaningful boundary | `game-session-provider.tsx`, `app-shell.tsx` | Provider ownership dan composition table/workspace; tidak membuat framework route/registry generik |
| Refactor owner | `use-table-session.ts`, `account-presence.tsx` | Satu socket+session; pisahkan transport menjadi hook internal hanya bila ownership/readability membutuhkan, jangan generic service layer |
| Ubah consumer | `bridge-table.tsx`, `lobby/lobby-client.tsx`, `profile-settings.tsx` | Session context, lifecycle actions satu jalur; BridgeTable tidak mengimpor feature Friends/Settings |
| Extend navigation | `app-navigation.tsx`, `(app)/layout.tsx` | Rail/bottom/More, real destinations, names/focus, active table indicator |
| Extend history | `table/trick-indicator.tsx`, `table/score-sheet.tsx`, `board-replay-modal.tsx` | Reader selection, leader/winner, responsive list, reuse authorized loaders |
| Buat workspace | `(app)/history/page.tsx` + `history-workspace.tsx` bila perlu client state | Compose existing score/match data; tidak membuat backend history baru |
| Extend result/status | `table/board-result.tsx`, `table/table-status-bar.tsx` | Explicit progression, compact score chain, status/turn/contextual actions |
| Primitive secukupnya | Native popover/dialog/button + existing `useDialogDrag`/`IssueNotice` | Shared tooltip/sheet hanya jika benar-benar digunakan beberapa fitur; tidak mengganti semua dialogs sekaligus |
| Styling | `globals.css` | Token typography/space/icon/target/focus/disabled dipakai konsisten; shared scale canonical, tanpa gradient/dependency |

Standardisasi loading/empty/error mengikuti pola satu area status + relevant action. Workspace loading tidak menyembunyikan table. Contoh: `History unavailable` + Retry; empty `Belum ada board`; reconnect status tidak menjadi toast tiap attempt. Bedakan reconnecting, offline/disconnected, session expired, dan table unavailable dari typed issue existing.

## State & Realtime Impact

1. **Reducer:** navigasi tidak masuk `reduceTableState`; authoritative/pending/presentation tetap terpisah. Preserve inflight requestId ketika membuka workspace; ACK/event menerima dan settle satu kali. Konflik/disconnect mengikuti rules existing, tanpa retry mutasi otomatis.
2. **Persisted session:** pertahankan key identity/access/table existing dan account validation. LocalStorage bukan tempat hand, projection, replay atau full history. Refresh boleh restore; menu switch tidak restore ulang. Logout/identity change juga menghapus marker table dan scoped workspace data.
3. **Socket ownership:** extend owner agar mampu connected tanpa table (account traffic), lalu subscribe setelah activation. AccountPresence tidak lagi membuka socket sendiri. Ketika explicit leave/switch perlu detach table dan protokol belum menyediakan unsubscribe yang sesuai, intentional close/reopen diperbolehkan; tidak perlu menambah command protocol hanya untuk itu.
4. **Reconnect/rotation:** retry/backoff, credential refresh single-flight, online/offline handling dan generation fence dipertahankan. Rotation 285 detik adalah lifecycle existing yang sah; jangan melabelinya regression navigation. Tidak ada timer baru per workspace render.
5. **Snapshot/revision:** snapshot hanya bootstrap/resume/gap/conflict/known resource invalidation. Navigasi biasa tidak mengirim `table.resume`, subscribe, takeover, snapshot fetch atau membuat expected_revision reset. Sequence gap dan stale controller tetap dihormati.
6. **Stale async work:** callbacks ticket/GET/profile/replay lama tidak dapat menulis ke table/account berikutnya. Tests mencakup close/switch selama request pending.
7. **Match completion entitlement:** source menyebut replay refreshed on table re-entry. Setelah table persistent, polling match yang bertransisi COMPLETE harus memicu satu refresh projection/capability authorized bila event table belum membawa `matchComplete`. Gunakan endpoint existing dengan dedup per transition/revision; jangan men-set permission true hanya dari route atau cache. Ini invalidation domain yang sah, bukan refetch setiap buka/tutup Match.
8. **Social/presence:** satu heartbeat 15 detik pada shell dan satu chatStore attachment. Navigasi tidak membuat online flicker/offline lease baru; inbound table/chat/social tetap dibedakan oleh envelope existing.
9. **Engine/API:** tidak ada engine rule/protocol/schema change yang direncanakan. Jika inspection implementation menemukan missing server signal yang benar-benar memblokir, buat follow-up/ADR dengan bukti; jangan memperluas projection history/room privacy secara diam-diam.

## Work Items

Semua acceptance di bawah adalah **belum dikerjakan**. Setiap item dapat menjadi issue/PR. Pisahkan commit sesuai boundary; pesan hanya `+ action: description`, tanpa attribution trailer.

### P6-01 — Kunci regression baseline dan session contract

Tujuan: membuat perubahan ownership dapat dibandingkan dengan behaviour existing.
Scope: characterization tests untuk route, socket/frame count, restore dan lifecycle; dokumentasikan route/ownership decision ini pada roadmap.
Dependency: tidak ada. Parallelization: fixture UX P6-05–07 boleh disiapkan setelah contract agreed; tidak perlu menunggu refactor selesai untuk audit fixtures.

- [ ] Test existing memperlihatkan table → workspace menyebabkan close/reopen/GET; pisahkan HMR socket dari `/v1/ws`.
- [ ] Fixture deterministic mencakup auction, opening lead, middle trick, Dummy/defender, scored, match active/complete.
- [ ] Rekam identity/revision/seq/pending dan connection count tanpa credential atau hidden hand dalam output umum.
- [ ] Tetapkan baseline raw-frame privacy, leave, takeover, claim/undo, replay dan chat yang wajib tetap lulus.

### P6-02 — Satukan GameSession dan realtime owner

Tujuan: table tidak bergantung lifecycle page.
Scope: provider, refactor hook account/table connection, consumer lobby/table/presence; single heartbeat/chat attach.
Dependency: P6-01. Parallelization: P6-05/06 dapat bekerja pada presentational components; file hook/provider dimiliki satu work item.

- [ ] Satu authenticated shell memiliki maksimal satu settled app socket dan satu subscription table aktif.
- [ ] Mount ulang consumer, profile update, dan workspace state change tidak membuat socket/pending reducer baru.
- [ ] No-table chat/social → join/subscribe → explicit leave kembali account realtime berfungsi tanpa competing attach.
- [ ] Stale async completion, strict effect cleanup, logout error, credential refresh, account switch dan storage-clear ditangani deterministik.
- [ ] Reducer/game protocol behaviour existing tetap lulus; route switching bukan trigger controller takeover.

### P6-03 — Persistent AppShell dan workspace routing

Tujuan: satu table surface bertahan di seluruh authenticated route.
Scope: shared layout, pemindahan route files tanpa ganti URL, table activation boundary, slot workspace dan per-workspace presentation state, error boundaries.
Dependency: P6-02. Parallelization: navigation visual P6-04 setelah interface shell/session stabil.

- [ ] Table → Friends → Settings → History → Table tidak mengganti table mount identity, socket, subscription atau bootstrap GET.
- [ ] Back/Forward, direct link Settings+active marker, close workspace dan refresh mengikuti lifecycle table di atas.
- [ ] Scroll/filter/draft/selected trick/board pulih bila masih valid; board/permission changes menghapus state yang invalid.
- [ ] Workspace fetch/error/loading tidak merusak table; profile `router.refresh()` tidak reset provider.
- [ ] Invite/table lain meminta explicit switch; failure leave tidak menghilangkan meja lama.

### P6-04 — Navigation, utilities dan copy

Tujuan: table mudah dicapai tanpa chrome berlebihan.
Scope: `AppNavigation`, More, profile link, contextual Match/Notifications/Help; reuse Friends/Settings forms dan chat.
Dependency: P6-03 shell contract. Parallelization: P6-05/06/07 pada components berbeda; koordinasikan satu perubahan globals.css.

- [ ] Desktop icon-first punya accessible names, tooltip focus/hover dan active workspace; mobile maksimal empat item dengan label.
- [ ] History destination bekerja; Deals/Rankings/fitur kosong tidak memunculkan placeholder utama.
- [ ] Table active/turn indicator cukup satu; tidak ada return banner duplikat.
- [ ] Settings/Profile tidak duplikat; save/mute/logout memakai state dan lifecycle shared.
- [ ] Tidak ada subtitle yang hanya mengulang heading; notices membuka workspace tanpa leave atau auto-join.

### P6-05 — Safe background gameplay dan readable table

Tujuan: table persistent tidak memicu input/motion/lifecycle tersembunyi.
Scope: visibility/focus contract, shortcut, pointer capture, motion/audio, explicit Next board; geometry kartu/table di dalam shell.
Dependency: P6-02; integrasi final sesudah P6-03/04. Parallelization: P6-06/07, karena engine/capabilities tetap shared existing.

- [ ] P/X/R, Enter, Escape dan pointer pada workspace tidak mengirim gameplay command; visible table tetap bisa dimainkan ketika focus/pointer sah.
- [ ] Satu click/tap/drag menghasilkan satu command; illegal/nonplayable known state mengirim nol command.
- [ ] Remote actions diterima saat workspace terbuka; return tanpa stale motion backlog atau audio duplikat.
- [ ] BoardResult tidak mengirim next_board dari 5-second timeout, klik dokumen atau perubahan visibility; Next board explicit capability-guarded.
- [ ] Own hand/dummy/current trick/seat readable dan tidak overlap di semua target; card selection di narrow touch tidak mengandalkan strip kecil.

### P6-06 — Auction dan trick history reader

Tujuan: sequence, leader, winner dan turn dapat dipahami pada mobile/desktop.
Scope: AuctionTable scroll policy, TrickIndicator presentation/selection, mobile panel dan current trick cue; tanpa perubahan entitlement.
Dependency: P6-01 fixtures, P6-03 integration. Parallelization: dapat dikerjakan terpisah dari score P6-07.

- [ ] Dealer/order/Pass/X/XX/contract/declarer dapat diakses tanpa prose panjang atau color-only cue.
- [ ] Mobile trick detail menampilkan nomor, seat tiap kartu, leader, play order dan authoritative winner tanpa horizontal scrolling.
- [ ] Dummy dapat prev/next authorized history; non-Dummy tidak menerima/menyimpan/reconstruct unauthorized older tricks.
- [ ] Incoming call/trick tidak mencuri reader scroll/selection; undo/board change melakukan clamp/reset yang tepat.
- [ ] Open/close/Escape/focus return berjalan pada keyboard/touch tanpa nested modal stack.

### P6-07 — History workspace dan score context

Tujuan: History menjadi destination nyata dengan sumber data yang tersedia.
Scope: compose ScoreSheet/replay + participant match list; responsive board rows dan result/score chain.
Dependency: P6-01 fixtures, P6-03 shell. Parallelization: P6-06 pada reader terpisah; koordinasi selected-board state.

- [ ] Active-table History memakai state valid tanpa full table GET baru; no-table menampilkan authorized match list/empty state.
- [ ] Contract/declarer/vulnerability/result/signed perspective, passed-out/zero/positive/negative terlihat jelas.
- [ ] Duplicate total tidak dilabeli IMP; Team Match memakai actual IMP/comparisons; tidak ada Matchpoint placeholder.
- [ ] Replay/analysis tetap ditolak selama active match; match COMPLETE menyegarkan entitlement sekali tanpa route-remount dependency.
- [ ] History panjang memakai scroll vertikal mobile dengan row readable, stable selection dan return ke live table.

### P6-08 — Accessibility dan responsive integration gate

Tujuan: membuktikan shell baru tidak mengurangi keterbacaan atau kemampuan bermain.
Scope: final token/primitive consistency, target/contrast/zoom/focus, safe area/keyboard, responsive composition; bukan broad CSS rewrite.
Dependency: P6-04–07. Parallelization: accessibility review dan geometry/pointer checks dapat dijalankan terpisah setelah baseline integration sama.

- [ ] Target 44 px, font baseline, contrast dan non-color states terukur; full long name dapat diakses.
- [ ] Keyboard navigation/modal focus/Escape/return, zoom 200%/400%, text enlargement, reduced-motion dan screen-reader announcements diverifikasi.
- [ ] Matrix desktop/laptop/tablet/320–390/landscape lulus geometry dan actual pointer checks, bukan screenshot saja.
- [ ] Error/loading/reconnecting states compact, tidak berulang tiap retry dan tidak mengganti surface table.

### P6-09 — Persistence regression dan handoff evidence

Tujuan: phase dapat dinyatakan selesai berdasarkan bukti behaviour.
Scope: browser regression terfokus, reducer/transport tests, existing privacy/engine gates sesuai impact; documentation hasil dan known limits.
Dependency: P6-02–08. Parallelization: suites independent boleh berjalan paralel dengan DB/ports terisolasi; tidak perlu parallel agent implementation.

- [ ] Semua skenario validation berikut mempunyai hasil, command, viewport, fixture/role dan artifact yang dapat direproduksi.
- [ ] Tidak ada reconnect/GET/subscribe/takeover akibat navigation dalam measurement window; planned rotation diuji terpisah.
- [ ] Existing multi-player, bot consensus, leave, chat, replay dan Team Match guards tidak regression.
- [ ] Root/web plan diperbarui berdasarkan bukti; Phase 5 release gates yang belum selesai tetap pending.

## Validation Strategy

### Oracle utama persistence

Gunakan Playwright dengan API/PostgreSQL lokal terisolasi dan empat akun untuk gameplay; delapan akun existing smoke untuk match. Hook `page.on('websocket')` hanya endpoint `/v1/ws`; hitung open/close dan outgoing subscribe/resume/takeover/mutation; hitung GET table serta ticket requests. Jangan menyimpan raw tickets/tokens/hands ke report publik.

Setelah bootstrap stabil, jalankan minimal 10 siklus Table → Friends → Settings → History → Table, termasuk Back/Forward. Dalam window tanpa network disruption, rotation atau domain invalidation: **delta socket open/close=0, subscribe/resume/takeover=0, GET full table=0, leave=0**. Table instance/mount marker tetap sama. Feature-specific REST seperti Friends search diperbolehkan.

Dengan game diam, boardId/hand/auction/current trick/score/revision sama. Dengan remote action saat workspace terbuka, revision/seq maju dan return cocok dengan latest authorized server projection. Pending local action lalu pindah workspace sebelum ACK harus settle sekali tanpa rollback palsu/duplikasi. Gunakan 500 ms delayed authoritative frames seperti fixture existing.

### Matrix skenario

| Skenario | Assertions minimum | Level termurah yang memadai |
|---|---|---|
| Ongoing auction | Bid pending + nav, P/X/R pada Friends nol command, return selection/legalCalls benar | Reducer/transport test + Playwright |
| Opening lead / ongoing trick | Card gesture lalu nav, remote play/undo, dummy entitlement, current trick latest | Existing four-player flow + geometry/mouse/touch |
| Finished board | Tunggu >5 detik dan klik Settings tidak advance; explicit Next board satu command; score/history persisten | Component/browser + authoritative revision assertion |
| History panjang | Dummy 13 trick/52 cards authorized; non-Dummy latest-only; ≥32 score rows fixture; long auction scroll | Real renderer fixture + raw-frame privacy baseline |
| Long names / scores | Nama 24 karakter, names ber-spasi dan unbroken; NS/EW ±/0, pass-out, X/XX; match pending/final | Presentation unit/fixture + screenshots |
| Back/Forward/refresh | Internal history tidak disconnect; refresh/direct link restore sekali, missing/expired marker ditangani | Browser with network instrumentation |
| Reconnect | Putus jaringan saat Settings, kembali online, snapshot/resume authoritative, pending semantics existing | Browser network control + reducer |
| Rotation / sleep | Rotation 285 detik secara terkontrol, resume setelah wake; tidak duplicate socket atau false leave | Transport fake clock + satu browser smoke |
| Auth/controller | Logout failure/success, expired auth, removal/table unavailable, account switch, newer-tab takeover | Session tests + browser two-context |
| Match | Return Match preserves room socket; complete refresh entitlement sekali; no cross-room leak | Existing eight-client/Team Match smoke |
| Notifications/chat | Private/table delivery, draft, follow/invite dedup; invite switch guard; unread bukan archive palsu | Existing chat/social tests + navigation smoke |
| Error/loading | Friends/History fetch gagal, disconnected vs expired, retry tanpa table splash/toast loop | Mock feature failure + browser |
| Input/accessibility | Keyboard, focus, touch tap/drag/cancel, zoom, reduced motion, screen-reader cue | Real browser/pointer + manual assistive-tech pass |

### Commands dan evidence

- Reuse `apps/web/e2e/phase3.spec.ts`, `table-leave.spec.ts`, `account.spec.ts`, `chat.spec.ts`, `team-match.spec.ts`, `board-replay-browser.spec.ts`, `trick-history-layout.spec.ts`; tambah `persistent-session.spec.ts` untuk assertions navigation yang belum ada.
- Web checks: `corepack pnpm --filter @bridgeyok/web test`, `typecheck`, `lint`, `build`. `corepack pnpm --filter @bridgeyok/web e2e` menjalankan config localhost 3100/8180 dan membutuhkan DATABASE_URL disposable yang dimigrasi; jangan arahkan ke produksi.
- Layout fixture config berguna untuk geometry history/replay tetapi **tidak** membuktikan session persistence. Browser E2E harus memakai rendered app dan server nyata untuk socket/lifecycle oracle.
- Tidak perlu menulis ulang engine tests untuk CSS/docs; bila ada perubahan API/projector yang terpaksa dilakukan, jalankan race/privacy/contracts suite terkait dan dokumentasikan alasan scope.
- Simpan screenshot/trace dan ringkasan counts redacted dengan baseline revision. Lakukan review nyata pada kartu, dummy, current trick, score/history dan zoom; automated no-overflow bukan pengganti review.
- Audit planning saat ini tidak mengklaim semua matrix ini telah dijalankan. Detail cakupan aktual ada di evidence audit.

## Objective of Done

- [ ] Membuka Friends/Settings/History saat table aktif tidak menutup atau membuka WebSocket, mengubah subscription, atau memanggil leave.
- [ ] Kembali ke Table memakai instance/session yang sama tanpa GET full game state atau loading splash ketika local session valid.
- [ ] Sepuluh siklus navigasi + Back/Forward memenuhi network counter oracle nol dalam window tanpa recovery/rotation.
- [ ] Board, hand, auction, trick, score dan revision tidak berubah karena navigation; remote actions tetap diterima saat workspace terbuka.
- [ ] Pending action melewati navigation dan ACK/event tepat satu kali; route change tidak menyebabkan takeover.
- [ ] Refresh/direct link pada table maupun workspace memulihkan identity/table authorized sekali; network loss dan planned rotation tetap pulih.
- [ ] Satu session owner, satu heartbeat dan satu settled app socket; account/table/chat tidak bersaing attach.
- [ ] Explicit leave/switch/logout terpisah dari menu navigation; failure tidak menghilangkan table secara palsu.
- [ ] Table default primary surface; desktop side panel hanya jika geometry aman; mobile Table selalu satu tap tanpa banner duplikat.
- [ ] Desktop primary navigation icon-first dengan accessible names dan tooltip seperlunya; mobile maksimal empat compact labeled items.
- [ ] History route benar-benar bekerja dari data existing, tanpa global archive/IMP/Matchpoint palsu.
- [ ] Trick history terbaca pada 320 px, menunjukkan seat/leader/order/winner sesuai data, dan mempertahankan recipient-scoped privacy.
- [ ] Score menyambungkan contract/declarer/vulnerability/result/perspective; duplicate vs actual Team IMP tidak tertukar.
- [ ] Finished-board timeout, workspace click dan animation timing tidak memajukan board; Next board eksplisit dan legal.
- [ ] Matched completion membuka authorized replay melalui refresh capability terarah, bukan remount table atau bypass guard.
- [ ] Scroll/filter/draft/selected history bertahan ketika relevan; undo/board/account/permission changes menghapus state stale.
- [ ] Hidden/inert table tidak menangkap shortcut/pointer, meninggalkan top-layer modal, atau menumpuk motion/audio lama.
- [ ] Cards/dummy/trick/players tidak overlap pada matrix; touch card/action targets usable tanpa mengecilkan font untuk fit.
- [ ] Semua icon-only actions bernama; focus/keyboard/modal close/return, contrast, 200%/400% zoom dan reduced motion lulus.
- [ ] Tidak ada explanatory subtitle berulang, gradient/dekorasi baru, secondary container bertumpuk, atau Coming soon primary destination.
- [ ] Loading/error/reconnecting compact dan tidak spam; workspace failure tidak menghancurkan game session.
- [ ] Existing gameplay/claim/undo/chat/replay/match/privacy regression lulus; tidak ada perubahan core bridge rules atau unreviewed protocol.
- [ ] Evidence ledger lengkap dan focused commits sesuai AGENTS.md; release gates lama tidak ditandai selesai tanpa bukti baru.

## Risks dan follow-up di luar scope

- Persistent table menyingkap global keydown, click-to-advance, top-layer dialog, stale motion dan duplicate chat ownership. P6-02/03 **tidak boleh dirilis sendiri** tanpa safety P6-05 dan regression gate.
- Match capability sebelumnya mengandalkan re-entry; P6-07 wajib menghapus dependency tersebut tanpa memperluas visibility sebelum match complete.
- Browser/OS dapat suspend connection; recovery tetap diperlukan. Shared layout tidak mengubah five-minute all-offline expiry server atau membenahi production multi-instance limitation.
- Global casual archive, durable notification inbox, rankings, matchpoints, dan game clock adalah follow-up product/backend terpisah jika kelak diminta.
- Tidak ditemukan blocker engine yang perlu dimasukkan dari audit ini. Temuan engine baru dicatat terpisah, kecuali terbukti langsung memblokir acceptance UX.
