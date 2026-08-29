# AGENTS.md

# Core Development Principles

## Preserve Existing Architecture

Before creating a new component, hook, utility, type, or helper:

1. Inspect whether a suitable implementation already exists.
2. Prefer extending an existing abstraction when doing so keeps the code readable.
3. Do not introduce a new abstraction solely to reduce a few lines of code.
4. Avoid unnecessary files and indirection.
5. Keep related logic close to where it is used unless there is a clear reuse or separation-of-concerns benefit.

Code should remain easy to trace from UI interaction to implementation.

---

# Naming Convention

Use `camelCase` for:

* variables
* functions
* methods
* parameters
* local constants that are not global configuration constants
* object properties when the project controls the naming

Examples:

```ts
const projectItems = [];
const selectedProject = null;

function openProjectModal() {}

function calculateExperienceDuration() {}
```

Use `PascalCase` for:

* React components
* TypeScript interfaces
* TypeScript types
* classes

Examples:

```ts
function ProjectCard() {}

interface ProjectItem {}

type ProjectCategory = "frontend" | "backend";
```

Global constants may use `UPPER_SNAKE_CASE` only when they are genuinely static configuration constants.

Example:

```ts
const MAX_VISIBLE_PROJECTS = 6;
```

---

# Index Variable Convention

Index variables must start with `_`.

Use:

```ts
items.map((item, _index) => ...)
```

```ts
for (let _index = 0; _index < items.length; _index++) {}
```

Do not use:

```ts
items.map((item, index) => ...)
```

For nested indexes, use descriptive variants when necessary:

```ts
sections.map((section, _sectionIndex) => {
  return section.items.map((item, _itemIndex) => ...)
});
```

---

# Comment Policy

Comments are intentionally restricted.

Do not create explanatory comments inside function bodies.

Do not create floating comments that do not belong directly to a function declaration.

Do not narrate implementation details using inline comments.

Avoid:

```ts
function calculateYears() {
  // Get the current year
  const currentYear = new Date().getFullYear();

  // Calculate total years
  return currentYear - 2024;
}
```

Avoid:

```ts
// This section handles project filtering
const filteredProjects = ...
```

Avoid comments that merely translate code into natural language.

Comments may only be used to document a function when the function requires clarification.

Function documentation must be placed immediately above the function and should contain only useful information such as:

* short function purpose
* parameters when their meaning is not obvious
* return value when clarification is useful

Prefer JSDoc.

Example:

```ts
/**
 * Calculates professional experience duration from a starting date.
 *
 * @param startDate Starting date of the experience.
 * @returns Human-readable experience duration.
 */
function calculateExperienceDuration(startDate: Date) {
  ...
}
```

Do not add JSDoc mechanically to every trivial function.

A self-explanatory function does not require documentation.

Example:

```ts
function closeModal() {
  setActiveModal(null);
}
```

No comment is needed.

The goal is:

> Code should explain implementation. Comments should explain intent only when intent cannot be expressed clearly through code.

---

# Function Design

Do not over-fragment logic into very small functions.

Avoid creating helper functions containing only approximately 3–5 lines when:

* they are used once
* their implementation is straightforward
* keeping the logic inside the main function improves traceability
* extracting them does not create meaningful semantic boundaries

Avoid patterns such as:

```ts
function getProjectName(project: Project) {
  return project.name;
}

function getProjectDescription(project: Project) {
  return project.description;
}

function isProjectFeatured(project: Project) {
  return project.featured;
}
```

when these operations can be read directly.

Prefer keeping local transformations within the primary function when they naturally belong there.

Extract a function when at least one of these conditions applies:

* logic is reused
* logic is complex enough to deserve an explicit name
* logic forms a meaningful domain operation
* extraction significantly improves readability
* logic needs isolated testing
* logic has independent side effects
* the function represents an important interaction or business rule

Do not optimize for the number of lines per function.

Optimize for readability and traceability.

---

# React Component Design

Do not over-componentize the interface.

A component should generally be extracted when:

* it is reused
* it represents a meaningful visual or behavioral unit
* it contains substantial independent logic
* separating it makes the parent significantly easier to understand

Do not create a component merely because a JSX block contains a few elements.

Avoid structures such as:

```text
Hero
├── HeroTitle
├── HeroSubtitle
├── HeroDescription
├── HeroButtons
├── HeroButton
├── HeroSocials
└── HeroSocialItem
```

unless those components genuinely have independent behavior or reuse.

Prefer:

```text
HeroSection
├── ProfileIntro
├── HeroActions
└── SocialLinks
```

or keep simple markup directly within `HeroSection`.

---

# Design System

## Global CSS Is the Source of Truth

Before adding any color, inspect:

```text
globals.css
```

especially variables declared under:

```css
:root
```

Existing CSS variables are the primary design tokens.

Always reuse those variables whenever possible.

Do not introduce arbitrary colors directly inside components when an equivalent design token already exists.

Avoid:

```tsx
className="bg-[#FBEFEF]"
```

when the project already defines an appropriate CSS variable.

Prefer semantic project tokens.

---

# Color Rules

Never use gradients.

This applies to:

* backgrounds
* buttons
* cards
* text
* borders
* overlays
* decorative elements
* hover states
* illustrations generated using CSS
* section transitions

Do not use:

```css
linear-gradient(...)
radial-gradient(...)
conic-gradient(...)
```

Do not use Tailwind gradient utilities such as:

```text
bg-gradient-to-*
from-*
via-*
to-*
```

unless those utilities are unrelated to an actual gradient, which should be extremely rare.

Use flat colors from the project's palette.

Additional colors may only be introduced when necessary and must visually harmonize with the existing `:root` palette.

---

# Visual Direction

The visual language should be:

* minimal
* elegant
* editorial
* intentional
* personal
* professional
* slightly playful where appropriate
* visually distinctive without becoming decorative noise

Avoid designs that resemble generic AI-generated landing pages.

Avoid excessive use of:

* glowing effects
* glassmorphism
* floating blobs
* random decorative circles
* excessive pills
* excessive badges
* excessive icons
* unnecessarily rounded containers
* huge shadows
* neon effects
* generic dashboards
* fake operating-system windows
* decorative code editors
* generic terminal cards
* fake browser chrome
* random grids purely for decoration

Whitespace, typography, hierarchy, composition, and subtle details should provide most of the visual identity.

---

# Card Design

Cards must remain minimal and purposeful.

Do not give ordinary cards fake desktop window controls.

Never add decorative controls such as:

```text
● ● ●
```

or:

```text
close
minimize
fullscreen
```

unless the component explicitly represents an actual window interface and the feature requires those controls.

Project cards, experience cards, achievement cards, skill cards, and profile cards should look like content containers rather than fake application windows.

Prefer:

* clear typography
* restrained border usage
* subtle background contrast
* controlled spacing
* a small number of deliberate accents
* meaningful hover interactions

Avoid stacking multiple nested cards unless hierarchy genuinely requires it.

---

# Border Radius

Do not make every element heavily rounded.

Use border radius consistently with the existing design system.

Avoid creating excessive combinations of:

* rounded cards
* rounded inner cards
* rounded badges
* rounded icons
* rounded buttons
* rounded image containers

within the same small visual area.

Variation in shape is encouraged when it improves composition.

---

# Shadows

Use shadows sparingly.

Do not use large generic SaaS-style shadows on every container.

Prefer:

* borders
* background contrast
* whitespace
* typography
* subtle elevation only where hierarchy needs it

---

# Animation and Interaction

Interactions should feel deliberate.

Good examples:

* subtle element movement
* small layout transitions
* contextual reveal
* modal expansion
* card response to pointer interaction
* tasteful hover state
* section-based motion

Avoid:

* excessive bouncing
* constant floating animation
* unnecessary parallax
* animation on every element
* large spring effects without purpose
* interactions that reduce readability
* interactions that exist only to demonstrate animation

Respect:

```css
prefers-reduced-motion
```

where appropriate.

---

# Modal System

Modals are a major visual element of this portfolio, but they must not dominate the entire layout.

Do not place all portfolio modal triggers into one dense Hero section.

Do not create a single cluster such as:

```text
Hero
├── About Modal
├── Experience Modal
├── Project Modal
├── Achievement Modal
├── Skills Modal
└── Contact Modal
```

The page should vertically introduce information through sections.

Modal triggers should appear in the section where their content contextually belongs.

Example:

```text
Hero
↓
About
↓
Experience
↓
Projects
↓
Achievements
↓
Skills / Toolkit
↓
Contact
```

A project modal should normally originate from the Projects section.

An achievement detail modal should normally originate from the Achievements section.

An experience detail modal should normally originate from the Experience section.

This gives each modal enough visual room and allows modal sizes to be larger without overwhelming the Hero.

---

# Modal Size

Do not force every modal into the same dimensions.

Modal size should follow content.

Possible variations:

```text
small
medium
large
wide
fullscreen-like
```

Large content such as project case studies may use a wide or tall modal.

Short contextual information should use a smaller modal.

Keep modal layout readable on mobile.

Do not create unnecessary nested modals.

---

# One-Page Portfolio Architecture

The primary portfolio experience should work as one vertically flowing page.

Do not turn every section into a separate route unless content complexity genuinely requires it.

The page should tell a story rather than behave like a dashboard.

Recommended structure:

```text
Page
│
├── Navigation
│
├── Hero
│
├── About / Profile
│
├── Journey / Experience
│
├── Selected Projects
│
├── Achievements
│
├── Skills / Toolkit
│
├── Personal / Beyond Code
│
├── Contact
│
└── Footer
```

---

# Section 1 — Navigation

Keep navigation compact.

Recommended contents:

* personal mark or name
* About
* Journey
* Projects
* Achievements
* Contact

Navigation should help movement through the page rather than visually dominate it.

Prefer anchor-based navigation for primary sections.

---

# Section 2 — Hero

The Hero must introduce the person, not the technology stack.

Primary goals:

1. establish identity
2. communicate professional direction
3. provide a memorable first impression
4. provide one or two clear actions

Recommended contents:

```text
HeroSection
├── Identity / Name
├── Primary Statement
├── Short Supporting Copy
├── Primary CTA
├── Secondary CTA
└── Small Personal Visual Element
```

Possible actions:

* Explore my work
* View projects
* Contact me
* Download résumé

Do not fill the Hero with many cards.

Do not place every section preview inside the Hero.

Do not make the Hero look like a dashboard.

---

# Section 3 — About / Profile

Purpose:

Explain who the person is beyond the Hero headline.

Recommended contents:

* concise personal introduction
* current focus
* professional interests
* small personal facts when relevant
* optional portrait or visual composition
* optionally one contextual modal for deeper profile information

Possible component structure:

```text
AboutSection
├── SectionHeading
├── AboutContent
└── ProfileDetails
```

Keep the primary information visible without requiring a modal.

---

# Section 4 — Journey / Experience

This section communicates progression over time.

Include:

* professional experience
* internships
* teaching or mentoring experience where relevant
* significant educational or collaborative experiences
* selected milestones

Prefer a strong vertical composition.

Possible structure:

```text
JourneySection
├── SectionHeading
├── JourneyTimeline
│   └── JourneyItem
└── ExperienceDetailModal
```

Do not make every timeline item unnecessarily large.

Detailed responsibilities or stories may open in contextual modals.

---

# Section 5 — Selected Projects

This is one of the primary sections.

Show a curated selection instead of dumping every repository.

Each project should communicate:

* project name
* project purpose
* role
* key challenge
* technologies
* outcome or impact
* visual preview when available

Recommended structure:

```text
ProjectsSection
├── SectionHeading
├── ProjectCollection
│   └── ProjectCard
└── ProjectDetailModal
```

Project cards should not resemble browser windows unless the design explicitly shows a real browser screenshot.

Project detail modals may be significantly larger than ordinary cards.

A project modal may contain:

```text
ProjectDetail
├── ProjectIntroduction
├── ProjectVisual
├── Problem
├── Approach
├── KeyTechnicalDecisions
├── Result
└── RelevantLinks
```

Do not show every technical detail on the initial card.

---

# Section 6 — Achievements

Use this section for achievements that strengthen the professional narrative.

Possible content:

* awards
* international collaborations
* competitions
* notable presentations
* technical accomplishments
* certifications when meaningful

Recommended structure:

```text
AchievementsSection
├── SectionHeading
├── AchievementCollection
│   └── AchievementItem
└── AchievementDetailModal
```

Avoid turning achievement items into generic certificate cards.

Focus on why the achievement matters.

---

# Section 7 — Skills / Toolkit

Do not create an enormous wall of technology badges.

Prefer grouped capabilities.

Example:

```text
Frontend
Next.js
React
TypeScript

Backend
Laravel
REST APIs

Infrastructure
Docker
Redis

Database
PostgreSQL
MySQL
SQL Server
```

Better yet, connect skills to actual experience.

Possible structure:

```text
SkillsSection
├── SectionHeading
├── SkillGroup
└── SupportingExperience
```

Avoid assigning fake percentage values such as:

```text
React 95%
Laravel 90%
SQL 87%
```

unless backed by an actual measurement system.

---

# Section 8 — Beyond Code

Optional, but useful for making the portfolio personal.

Possible contents:

* teaching
* robotics
* collaboration
* interests
* personal development
* working philosophy

Keep it professionally relevant.

This section should make the portfolio feel like a person's website rather than a résumé rendered into HTML.

---

# Section 9 — Contact

Keep contact simple.

Recommended contents:

* short closing statement
* email
* LinkedIn
* GitHub
* relevant social links
* résumé link if appropriate

Possible component:

```text
ContactSection
├── ClosingStatement
├── ContactActions
└── SocialLinks
```

Avoid unnecessarily complicated contact forms unless the project actually needs one.

---

# Section 10 — Footer

Keep the footer understated.

Possible contents:

* name
* current year
* selected navigation links
* small personal phrase

Do not repeat the entire navigation or contact section.

---

# Section Composition

Not every section must use cards.

Use different compositions across the page.

For example:

```text
Hero
large typography + visual composition

About
editorial two-column layout

Journey
vertical timeline

Projects
large project blocks

Achievements
structured list or asymmetric composition

Skills
grouped typography

Contact
minimal closing section
```

This variation is important.

A page where every section consists of:

```text
Heading
↓
3 rounded cards
↓
Heading
↓
3 rounded cards
```

should be avoided.

That pattern frequently creates a generic template or AI-generated appearance.

---

# Responsive Design

Design mobile-first.

Every major interaction must remain usable on small screens.

Do not rely on hover as the only way to access information.

Modal content must remain scrollable and readable.

Large desktop compositions should gracefully collapse instead of being replaced with completely unrelated mobile structures.

Avoid arbitrary responsive breakpoints when existing project breakpoints are sufficient.

---

# Accessibility

Use semantic HTML whenever possible.

Maintain:

* heading hierarchy
* keyboard accessibility
* visible focus states
* accessible modal behavior
* appropriate button semantics
* meaningful alt text
* sufficient color contrast
* reduced-motion support where applicable

Do not use clickable `<div>` elements when `<button>` or `<a>` is semantically correct.

---

# SEO

SEO is a first-class requirement.

Use Next.js metadata capabilities appropriately.

Important pages and content should provide meaningful:

* title
* description
* canonical information where required
* Open Graph metadata
* Twitter/X metadata when relevant

Use semantic content that search engines can understand.

Important portfolio information must not exist exclusively inside client-side modals.

Critical information such as:

* name
* professional role
* project names
* experience
* achievements

should exist in crawlable page content.

Modals may provide additional detail, but should not be the only source of important information.

Use structured data when appropriate, especially schema types relevant to:

* Person
* WebSite
* CreativeWork / SoftwareApplication when appropriate
* BreadcrumbList only when actual navigation hierarchy warrants it

Do not add structured data merely for the sake of adding SEO markup.

---

# Next.js Guidelines

Prefer Server Components unless client-side behavior is required.

Add:

```ts
"use client";
```

only when the component genuinely requires:

* React state
* effects
* browser APIs
* event-driven client behavior
* client-only libraries

Do not convert large component trees into Client Components merely because one small interaction requires state.

Move interactive boundaries downward when practical.

Use Next.js primitives when appropriate:

* `next/image`
* `next/link`
* metadata APIs
* route-level loading/error boundaries where useful

Avoid unnecessary runtime JavaScript.

---

# TypeScript

Keep TypeScript strict.

Do not introduce `any` unless no practical alternative exists and there is a clear technical reason.

Prefer inference when the type is obvious.

Do not create redundant interfaces for simple local objects unless the type provides reuse or meaningful domain clarity.

Reuse existing project types before introducing new ones.

---

# Styling

Follow the project's existing styling strategy.

Prefer existing CSS variables and design tokens.

Avoid arbitrary values unless necessary for an intentional composition.

Do not introduce a second design system.

Do not add a component library simply to implement basic UI already supported by the project.

---

# Dependencies

Do not add a new dependency when the requirement can be implemented cleanly using:

* existing project dependencies
* React
* Next.js
* browser APIs
* existing utilities

Before installing a package, determine:

1. why it is necessary
2. whether the project already has an equivalent
3. whether the package size and maintenance cost are justified

Avoid dependency-heavy solutions for minor visual effects.

---

# Refactoring Rules

When editing existing code:

* preserve unrelated behavior
* avoid broad cleanup unrelated to the task
* do not rename large areas of the codebase without reason
* do not rewrite working components simply to match a preferred style
* keep changes scoped to the requested feature

If a nearby issue materially prevents a correct implementation, fix it only when necessary.

---

# Plan Usage

`PLAN.md` is the implementation roadmap.

Before implementing significant new sections or redesigns:

1. read `PLAN.md`
2. understand completed and pending work
3. keep implementation aligned with the current plan
4. update architecture only when the requested change genuinely requires it

`AGENTS.md` defines permanent project rules.

`PLAN.md` defines what is currently being built.

Do not mix temporary implementation tasks into `AGENTS.md`.

---

# Decision Priority

When instructions appear to conflict, use this priority:

1. explicit user request
2. `AGENTS.md`
3. `PLAN.md`
4. existing project architecture
5. existing design system
6. general framework conventions

When uncertain, prefer consistency with the existing project over introducing a new pattern.

---

# Commit Policy

Partition implementation work into multiple focused commits whenever making changes.

Each commit message must contain only one or more lines in this format:

```text
+ action: description
+ action: description
```

Use a concise lowercase action that describes the change. Do not add a subject line,
body text, or metadata outside this format.

Never add attribution trailers, including `Co-authored-by`, to a commit message.

---

# Final Quality Check

Before considering a task complete, verify:

### Code

* naming follows the project convention
* index variables start with `_`
* no unnecessary atomic helper functions were introduced
* no unnecessary components were introduced
* no floating or implementation-narration comments were added
* TypeScript remains valid

### Design

* colors originate from `globals.css` tokens where possible
* no gradients were introduced
* no fake window navigation controls were added to cards
* cards remain minimal
* layout does not resemble a generic AI-generated dashboard
* interactions have a clear purpose
* desktop and mobile compositions remain usable

### Architecture

* modals are located contextually within relevant sections
* modals are not clustered entirely inside the Hero
* sections retain a clear vertical narrative
* important SEO content remains visible and crawlable outside modals

### Next.js

* Server Components remain the default
* Client Components are used only where necessary
* metadata and semantic HTML remain correct
* unnecessary JavaScript and dependencies were avoided

### Scope

* requested work is complete
* unrelated code was not unnecessarily modified
* implementation remains easy to trace
* changes were partitioned into multiple focused commits
* every commit message contains only `+ action: description` lines
* no commit message contains a `Co-authored-by` trailer
