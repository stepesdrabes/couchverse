# Native clients: progress

Tracks `docs/native-clients-plan.md` phase by phase on the `native-clients` branch. Updated as work
lands; the plan stays the design, this file says where things stand.

Legend: done, in progress, pending, blocked (needs the user).

## Phase 0: cleanup and restructure - done

Music removed, `frontend/` moved to `clients/web/`, CI for backend, web and the repo.

## Phase 1: contract - done

huma on the router, every route an operation, `contract/openapi.json`, the couch protocol schema
and golden frames, i18n in `contract/i18n` with plurals, design tokens, the generators
(`cargo xtask codegen`: Rust API crate, web typed client, Apple/Android strings and tokens), the web
admin on the typed client, typed enums on response fields, upload bodies, path builders.

## Phase 2: backend platform for devices - done

Server identity, device sessions with sliding expiry, pairing, connect codes, devices list, media
and artwork grants, JIT stop. Web: `/pair`, Connect-a-device QR, Devices list. Exit verified with a
scripted client (ffprobe and AVFoundation playing a grant URL).

## Phase 3: core foundation and first Apple slice - in progress

- Done: the core (`servers`, `accounts`, `session`, `theme`, `markdown`, plus the later modules
  below), bindings and packaging (`make core-apple|core-android|core-wasm`, `core.yml`,
  `android.yml`), the wasm budget (400 KB gzip, size build), the web on the core's session and
  markdown (no `{@html}`), the Android `core` and `design` Gradle modules with JVM tests.
- In progress: the Apple project, `CoreRuntime` and executors, design system v0, onboarding,
  pairing, Who's watching, Settings (agent branch `worktree-agent-a66ffee4e3976782d`).
- In progress: the Playwright web smoke suite (`worktree-agent-aed95c8a2bb6f34a9`).
- Blocked: spike S1 (free personal team capabilities) needs the user's Apple ID and devices.

## Phase 4: playback v2 - in progress

Probe enrichment, device-profile decision, remux tier, fMP4 HLS with renditions and I-frame
playlists, own HLS validator, AVFoundation harness, web on the new decision
(`worktree-agent-a0d18ff9feefb377c`, contract settled, docs and Docker checks left).

## Phase 5: browse and titles - in progress

- Done: core `catalog` (stale-while-revalidate, warm start, search, My List, image URLs), title
  logos (backend, TMDB, admin), logos in the core's views.
- In progress: the web's catalog pages on the core (`worktree-agent-a63057a70b9134335`).
- Pending: Apple browse and titles (after the Apple Phase 3 slice).

## Phase 6: playback - in progress

- Done: core `playback` (player effect, resume, watched-time accounting, progress saves, JIT
  keepalive and stop, preparing poll, qualities, audio and subtitle choices, next episode,
  shuffle, reload on failure).
- Pending: the core's device profile and `POST /playback` (after Phase 4 merges), the web
  player on the core, the Apple player.

## Phase 7: couch - in progress

- Done: couch v2 backend (participant tokens per device, `delivery=body` + `X-Couch-Token`,
  remote role and relay), core `couch` (socket effect, reconnect, host broadcast, follower
  drift sync, remote control, reactions).
- Pending: web couch on the core, Apple couch.

## Phase 8: ranks and profiles - in progress

- Done: core `ranks` and `profile` (rank badge, celebrations, profiles with heatmap,
  leaderboards, edits, uploads through the upload effect).
- Pending: web and Apple adoption.

## Phase 9: Apple system integration - pending

Top Shelf, widget, Live Activity (per spike S1), App Intents, Shortcuts, Spotlight.

## Phase 10: downloads - pending

Backend download preparation, core `downloads`, iOS downloads UI and offline mode.

## Phase 11: Android foundation - in progress

`CoreRuntime` and executors, design (phone and TV), onboarding, accounts, Who's watching, browse
and titles (`worktree-agent-ad92f5029f15e7764`).

## Phase 12: Android playback, couch, ranks, downloads - pending

## Phase 13: polish and release - pending

Accessibility and localization audit, Raspberry Pi performance, build-from-source guides,
release automation, docs, an HTML overview with screenshots of every client.

## Done outside the plan

- The web build is served precompressed (brotli/gzip), the wasm core 232 KB instead of 885 KB.

## Real-device checklist (for the user)

Collected from the phases as they land; each item is verified on simulators/emulators/Docker
first.

- Apple TV 4K: pairing from an iPhone, Who's watching animation, HDR/Atmos playback (Phase 6).
- iPhone: QR scan of a Connect-a-device code, Keychain persistence across reinstalls.
- Free personal team: 7-day provisioning, at most 3 apps, wireless pairing to the TV (spike S1).
- Google TV device: pairing, D-pad focus, Keystore persistence.
- Raspberry Pi 4 server: HLS v2 with the V4L2 encoder (spike S3).
