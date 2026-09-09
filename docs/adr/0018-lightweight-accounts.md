# ADR 0018 — Lightweight accounts before Team Match

Accepted 9 September 2026. Supersedes guest-only identity and friend-list exclusions in PLAN sections 1–5 and earlier identity decisions only for this scope.

Registered users have a stable public UUID, unique case-insensitive username, editable display name, password verifier and one predefined avatar. No uploads, OAuth, recovery email, public profile pages or community system. Passwords use Go standard-library PBKDF2-HMAC-SHA256 with random salt; bearer session verifiers are hashed and independently revocable. Password attempts are bounded per username and globally per minute, with at most four concurrent password operations. Account realtime tickets carry the login verifier binding; unused tickets are deleted on logout and established connections revalidate on command and ping. A stable internal guest identity preserves existing table ownership and reconnect boundaries. Public IDs never expose that internal identity.

Next.js server route guards validate a same-origin HttpOnly account cookie against the API before rendering public or protected pages. The existing realtime client obtains a compatible credential envelope through a same-origin endpoint; gameplay authority/projection and ticket exchange remain unchanged. Legacy guest APIs remain for compatibility with existing tables/tests, but the product web entry requires registration.

Directed follows are unique and idempotent. Friends means both directed edges exist. Search is authenticated and bounded. Participant profile lookup requires current table membership. Social online means an unexpired authenticated server heartbeat lease (45 seconds, refreshed every 15 seconds), across any login. Table socket presence remains authoritative for table connectivity.

Invites require current table membership, a verified invite code for that table, and a live recipient lease at insertion. They expire after ten minutes, are deduplicated per sender/recipient/table, and grant no seat or gameplay rights. Acceptance uses the existing join endpoint, including lock/capacity checks. Inbox polling does not expose other tables or hidden cards.

Home has four actual capabilities: Casual Game, Team Match, VS Robot, Teacher Table. Only Casual Game opens a flow. No arbitrary fifth feature; the grid accommodates future additions. History and Deals remain Coming soon actions with no destinations. Profile and Friends are the only additional app pages. The gameplay viewport retains its existing shell; the main sidebar/bottom navigation applies outside the table.

Implementation and verification progress: PLAN.md refinement section. Team Match implementation is explicitly excluded.
