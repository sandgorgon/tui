# Proposal: let an explicit Key survive a subtree moving to a new parent

Status: **accepted and implemented**, `a48b9d6` on `develop` (not yet
released). Filed as [#3](https://github.com/sandgorgon/tui/issues/3)
by the maintainer, surfaced building 9sh (a pane-splitting terminal
multiplexer) on top of `tui.App`. Implemented exactly as proposed
below: the existing `Key()` reused as a whole-tree fallback, no new
API. See `docs/DESIGN.md`'s reconciler changelog entry for the
implementation writeup.

## Problem

`reconcile.go`'s key matching is strictly **per-parent, one level at a
time**. `reconcileChildren` (`reconcile.go:102`) builds its `byKey` map
only from `prev`'s own direct children (`reconcile.go:104`), and
`reconcile` (`reconcile.go:57`) treats any tree slot that didn't match
locally as `prev == nil`, building a fresh retained node — including a
fresh `Widget` for `kindWidget` — regardless of what `Node.Key` the new
slot carries. Disposal of the old, unmatched subtree
(`disposeTree`, `dispose.go:18`) happens immediately, inline, as part of
that same per-parent pass (`reconcile.go:64` and `reconcile.go:127`) —
not deferred to end-of-frame.

Confirmed directly against the code (not assumed from the issue
report): if a leaf `L` keyed `"pane-1"` is a direct child of Box `P`
this frame, and next frame `L` needs to move one level deeper — wrapped
in a brand-new Box `S` that also holds a new sibling, with `S` now
occupying `L`'s old slot under `P` — `P`'s `reconcileChildren` sees `S`
as an unseen key, builds it fresh with `prev == nil`, and *inside* `S`,
`L` also starts from `prev == nil`. `L`'s own `.Key("pane-1")` is never
consulted, because nothing at `S`'s level knows to look for it — key
matching only ever searches one parent's own previous children list.

This matches `reconcileChildren`'s doc comment exactly as written
("reordering, inserting, or removing siblings doesn't disturb an
unrelated sibling's retained state") — the doc comment never claims to
handle a *parent* change, so this isn't a bug against the current
contract. It's a real gap against what any tiling/rearrangeable-layout
consumer needs: splitting a pane that hosts a live `widget.Terminal`
kills and restarts its pty, because the pane's Box always has to gain
one level of nesting the moment it's split (there's no sibling-level
key it could already be pre-registered under before the first split
happens — see the issue body for why "give the wrapper the old leaf's
key" doesn't work either, the wrapper's own *children* are structurally
incompatible with the leaf's).

Two more existing entry points into `reconcile` matter for scoping any
fix: `Tree.Reconcile` (`tree.go:18`) and `focusableWidget.Reconcile`
(`focus.go:47`) both call `reconcile` directly on their own
independently-retained subtree — a `widget.Viewport` or future
`widget.Modal` hosting content via `Tree`, or any `Focusable`-wrapped
child, is its own separate reconcile call, not part of the outer
`Box`/`reconcileChildren` walk. Whatever fix is chosen has to either
live within a single top-level `reconcile` call's own tree (the natural
boundary — matches how disposal already works, since `Tree.Close`
disposes independently too) or explicitly decide to reach across those
boundaries, which is a materially bigger change.

## Proposed design

Reuse the existing `Node.Key()` the reporter already sets on `L` in the
repro — no new exported API. Widen where a key is allowed to match,
as a fallback only, after the existing per-parent check fails:

1. **Per-parent match stays exactly as-is and is tried first.**
   `reconcileChildren`'s current `byKey` lookup is unchanged — this
   proposal changes nothing about the common case, and every existing
   reconcile test keeps passing unmodified.

2. **On a local miss, consult a whole-tree key index built from `prev`
   before any disposal happens.** At the start of a top-level
   `reconcile` call (`app.go:99`, `tree.go:18`, `focus.go:47` — each of
   these becomes the root of its own index), walk the previous retained
   tree once and record every keyed node found at any depth, not just
   at the top. When a `next` slot carries a key that misses the local
   per-parent map, check this index next; if found, reuse that retained
   node (and its `Widget`) as `prev` for this slot instead of building
   fresh.

3. **Disposal moves from immediate to deferred.** This is the real
   invariant change. Currently a `Box` disposes its own unmatched
   children the moment `reconcileChildren` returns
   (`reconcile.go:125`-`129`) — correct today because nothing outside
   that Box could ever want that subtree back. Once a keyed subtree can
   be claimed by *any* slot in the whole top-level tree, a parent can no
   longer know at its own level whether an unmatched child is truly
   gone or just moved elsewhere and not yet visited. So: collect
   unmatched-locally retained nodes into the index instead of disposing
   them inline, and only dispose whatever is left in the index once the
   entire top-level `reconcile` call finishes walking `next`.

4. **The existing kind/props-type mismatch check
   (`reconcile.go:58`-`61`) still applies** to an index-fallback match
   exactly as it does to a local match — reusing `L`'s retained node
   into a slot whose `next.kind` or widget-props type no longer matches
   `L`'s previous kind is exactly the Tabs-style "different widget, same
   position" case §3.1/M12 already guards against, and must stay
   guarded here too.

Net effect on the reporter's repro: `S`'s `reconcileChildren` still
finds no local match for `L`'s key among `P`'s old children (there is
none — `L` was `P`'s child, not `S`'s), but the whole-tree index built
from `P`'s previous subtree does have `L` under `"pane-1"`, so `L`'s
retained node — and its live `widget.Terminal`, uninterrupted pty
included — is reused in `S` instead of rebuilt.

## Scope / non-goals

- No new `Node` method, no new option on `Key()` itself — same key,
  wider search, opt-in by construction (only keyed nodes ever
  participate; unkeyed position-matching in `reconcileChildren` is
  untouched).
- Not proposing this reach across the `Tree`/`Focusable` boundaries
  listed above — a `widget.Viewport`'s hosted content and the outer
  `App` tree stay two separate index scopes, matching how disposal
  already treats them as independent today (`Tree.Close`,
  `dispose.go`).
- Not proposing any change to `reconcileChildren`'s existing unkeyed
  (position-based) fallback matching.

## Open questions

- **Collision risk.** Today a key only has to be unique among its own
  parent's children — a global index makes an accidental key collision
  between two unrelated parts of the tree (e.g. two independently-built
  panes that happen to both use `"pane-1"` as a locally-meaningful id)
  silently reuse the wrong subtree instead of erroring. Worth deciding
  whether this is acceptable as-is (matches how unkeyed collisions
  already silently misbehave in other frameworks' equivalent
  mechanisms) or needs a debug-mode check.
- **Cost of the whole-tree walk.** Building the index is an extra
  O(retained tree size) walk every frame, on top of the reconcile walk
  itself, for every `App`/`Tree` reconcile — worth measuring against
  M9's render-loop perf work rather than assumed to be negligible,
  particularly for a `Tree`-heavy app with many small `Viewport`s each
  paying their own index-build cost.
- **Whether deferred disposal changes visible timing anywhere.** Today
  an `io.Closer` widget's `Close()` runs synchronously inside the same
  `reconcileChildren` call that dropped it; under this proposal it runs
  at the end of the top-level reconcile instead — still within the same
  frame, but worth confirming nothing (a `Terminal`'s pty teardown, in
  particular) depends on the more specific *current* ordering.
