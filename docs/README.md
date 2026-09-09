# BridgeYok

BridgeYok is a web-based **Duplicate Contract Bridge** platform focused on realtime play, correct bridge mechanics, and built-in analysis.

The project is designed around a deterministic bridge engine, realtime multiplayer tables, persistent game state, and Double Dummy Solver (DDS) integration.

## Features

* Realtime multiplayer bridge tables
* Duplicate Bridge scoring
* Complete auction and card-play legality
* Declarer, dummy, trick, vulnerability, and dealer handling
* Reconnect and state recovery
* Persistent game history
* Board history with per-card navigation
* Double Dummy analysis
* DDS trick prediction for legal card plays
* Makeable contracts and par result
* Prepared and generated deals
* Team Match support
* Responsive desktop and mobile interface

## Tech Stack

**Frontend**

* Next.js
* React
* TypeScript

**Backend**

* Go
* PostgreSQL
* WebSocket

**Analysis**

* [dds-bridge/dds](https://github.com/dds-bridge/dds)
* Standalone C++ DDS executable

**Deployment**

* Vercel
* Docker
* Supabase / PostgreSQL

## Project Structure

```text
bridgeyok/
├── apps/
│   ├── api/          # Go API, bridge engine, realtime and persistence
│   └── web/          # Next.js frontend
├── scripts/          # Build and development scripts
├── tools/
│   └── dds/          # DDS wrapper
├── bin/              # Generated local binaries
├── go.work
└── Dockerfile.vercel
```

## DDS

BridgeYok uses a standalone DDS executable rather than linking DDS directly into the Go application.

Build it with:

```bash
./scripts/build-dds.sh
```

The generated binary is:

```text
./bin/bridgeyok-dds
```

The API uses the following configuration by default:

```env
DDS_EXECUTABLE=./bin/bridgeyok-dds
DDS_TIMEOUT=10s
DDS_CONCURRENCY=1
```

The DDS build script downloads a pinned revision of `dds-bridge/dds`, verifies its checksum, compiles the required C++ sources, and produces the runtime executable.

## Local Development

### Requirements

* Go 1.27+
* Node.js
* pnpm
* PostgreSQL
* g++
* curl

Clone the repository and install dependencies:

```bash
git clone https://github.com/mahesayuztar/bridgeyok.git
cd bridgeyok
```

Build DDS:

```bash
./scripts/build-dds.sh
```

Install frontend dependencies:

```bash
pnpm install
```

Configure the required environment variables, including:

```env
DATABASE_URL=
AUTH_SECRET=
ALLOWED_ORIGINS=http://localhost:3000
```

Then run the API and web application using the appropriate workspace development commands.

## Architecture

BridgeYok keeps bridge rules separate from infrastructure.

```text
Client
  │
  ├── HTTP API
  └── WebSocket
         │
         ▼
      Go API
         │
   ┌─────┼──────────────┐
   │     │              │
Bridge  Realtime     Persistence
Engine  Tables       PostgreSQL
   │
   ▼
Analysis Boundary
   │
   ▼
bridgeyok-dds
   │
   ▼
DDS
```

The bridge engine owns deterministic game rules and legality, while DDS is treated as an external analysis capability.

## Deployment

The API can be deployed using `Dockerfile.vercel`.

The Docker build:

1. Builds the DDS C++ executable.
2. Builds the Go API.
3. Copies both binaries into the runtime image.
4. Installs the required C++ runtime libraries.
5. Runs the API as a non-root user.

Runtime layout:

```text
/workspace/
├── bridgeyok-api
└── bin/
    ├── bridgeyok-dds
    └── dds-LICENSE
```

## Development Principles

* Bridge rules remain deterministic and testable.
* Illegal mechanical actions are rejected before becoming game state.
* Infrastructure concerns stay outside the pure bridge engine.
* Server state is authoritative.
* Realtime updates support reconnect and recovery.
* DDS analysis does not mutate game state.
* Completed boards remain reproducible from persisted data.

## License

BridgeYok source code is subject to the license defined by this repository.

DDS is distributed separately under its original license. See:

```text
bin/dds-LICENSE
```
