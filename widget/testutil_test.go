package widget

import (
	"testing"

	"github.com/sandgorgon/tui/cell"
	"github.com/sandgorgon/tui/tui"
)

// paintNode reconciles node fresh (as if it were the first frame) and
// paints it into buf, for tests that only care about a single frame's
// output. Tests exercising retained state across frames use tui.Tree
// directly instead.
func paintNode(t *testing.T, node tui.Node, buf *cell.Buffer) {
	t.Helper()
	var tr tui.Tree
	tr.Reconcile(node)
	tr.Paint(cell.NewPainter(buf))
}

// newTree returns a tui.Tree already reconciled once with node, for
// tests that need to reconcile it again later (e.g. to check retained
// state survives an update) and so can't use paintNode's throwaway
// Tree.
func newTree(t *testing.T, node tui.Node) *tui.Tree {
	t.Helper()
	tr := &tui.Tree{}
	tr.Reconcile(node)
	return tr
}
