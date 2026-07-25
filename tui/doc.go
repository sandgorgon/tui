// Package tui is the application and component model: an Elm-style
// App with Init/Update/View, where View returns a declarative Node
// tree that is diffed and reconciled against retained widget
// instances. Ephemeral UI state (scroll position, cursor blink,
// in-progress edits) lives in the retained widgets; application state
// lives in the caller's Model. Async work is expressed as Cmd values
// fed back into the single event-loop goroutine as Msg.
//
// See docs/DESIGN.md §3.1 for the full rationale.
package tui
