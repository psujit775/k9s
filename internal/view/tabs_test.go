// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"context"
	"testing"

	"github.com/derailed/k9s/internal/config/mock"
	"github.com/derailed/k9s/internal/model"
	"github.com/derailed/k9s/internal/view/cmd"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

func newTabsApp(t *testing.T) *App {
	t.Helper()
	a := NewApp(mock.NewMockConfig(t))
	require.NoError(t, a.Init("test", 0))
	return a
}

func TestTabsInit(t *testing.T) {
	a := newTabsApp(t)

	assert.Equal(t, 1, a.tabs.Count())
	assert.Same(t, a.Content, a.tabs.CurrentStack())
	assert.Equal(t, 0, a.tabs.active)
}

func TestTabsAddAndSwitch(t *testing.T) {
	a := newTabsApp(t)
	orig := a.Content

	tab := a.tabs.add()
	assert.Equal(t, 2, a.tabs.Count())
	assert.Same(t, orig, a.Content, "adding a tab must not switch")

	a.tabs.SwitchTo(a.tabs.Count() - 1)
	assert.Equal(t, 1, a.tabs.active)
	assert.Same(t, tab.stack, a.Content)
}

func TestTabsSwitchStopsOutgoing(t *testing.T) {
	a := newTabsApp(t)

	s0 := &tabComp{name: "s0"}
	a.Content.Push(s0)

	a.tabs.add()
	a.tabs.SwitchTo(a.tabs.Count() - 1)
	assert.GreaterOrEqual(t, s0.stopped, 1, "outgoing top must be stopped")

	startsBefore := s0.started
	a.tabs.SwitchTo(0)
	assert.Greater(t, s0.started, startsBefore, "returning to a tab restarts its top")
	assert.Same(t, s0, a.Content.Top())
}

func TestTabsSwitchToEmptyOpensPrompt(t *testing.T) {
	a := newTabsApp(t)
	a.Content.Push(&tabComp{name: "pods"})
	a.tabs.add()

	a.tabs.SwitchTo(1)
	assert.True(t, a.CmdBuff().IsActive(), "landing on an empty tab activates the command prompt")
}

func TestTabsCloseDisposesWithoutRestartCascade(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()
	a.tabs.SwitchTo(1)

	s1, s2 := &tabComp{name: "s1"}, &tabComp{name: "s2"}
	a.Content.Push(s1)
	a.Content.Push(s2)
	a.tabs.SwitchTo(0) // background the depth-2 tab

	s1Starts, s2Starts := s1.started, s2.started
	a.tabs.Close(1)

	assert.Equal(t, s1Starts, s1.started, "closing a tab must not re-Start() intermediate views")
	assert.Equal(t, s2Starts, s2.started)
	assert.Equal(t, 2, s1.stopped, "each view is stopped once on push-off, once on dispose")
	assert.Equal(t, 2, s2.stopped)
}

func TestTabsSwitchRestoresStack(t *testing.T) {
	a := newTabsApp(t)

	s0a, s0b := &tabComp{name: "s0a"}, &tabComp{name: "s0b"}
	a.Content.Push(s0a)
	a.Content.Push(s0b)

	a.tabs.add()
	a.tabs.SwitchTo(a.tabs.Count() - 1)
	a.tabs.SwitchTo(0)

	assert.Equal(t, []model.Component{s0a, s0b}, a.Content.Peek())
	assert.Equal(t, "[#000000:#00ffff:b] <s0a> [-:#000000:-] [#000000:#ffa500:b] <s0b> [-:#000000:-] \n",
		a.Crumbs().GetText(false))
}

func TestTabsCloseMiddle(t *testing.T) {
	a := newTabsApp(t)
	t1 := a.tabs.add()
	a.tabs.add()
	a.tabs.SwitchTo(2)

	a.tabs.Close(1)
	assert.Equal(t, 2, a.tabs.Count())
	assert.Equal(t, 1, a.tabs.active, "active index shifts down after closing an earlier tab")
	assert.NotContains(t, a.tabs.tabs, t1)
}

func TestTabsCloseActive(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()
	a.tabs.add()
	a.tabs.SwitchTo(1)

	a.tabs.Close(1)
	assert.Equal(t, 2, a.tabs.Count())
	assert.Equal(t, 1, a.tabs.active)
	assert.Same(t, a.tabs.tabs[1].stack, a.Content)
}

func TestTabsCloseLastRejected(t *testing.T) {
	a := newTabsApp(t)

	a.tabs.Close(0)
	assert.Equal(t, 1, a.tabs.Count())
}

func TestTabsSwitchBlockedByDialog(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()

	a.Content.AddPage("d", tview.NewModalForm("t", tview.NewForm()), true, true)
	require.True(t, a.Content.IsTopDialog())

	a.tabs.SwitchTo(1)
	assert.Equal(t, 0, a.tabs.active, "must not switch while a dialog is up")
}

func TestTabsCycle(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()
	a.tabs.add()

	a.tabs.Next()
	assert.Equal(t, 1, a.tabs.active)
	a.tabs.Next()
	assert.Equal(t, 2, a.tabs.active)
	a.tabs.Next()
	assert.Equal(t, 0, a.tabs.active, "next wraps around")
	a.tabs.Prev()
	assert.Equal(t, 2, a.tabs.active, "prev wraps around")
}

func TestTabsJump(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()
	a.tabs.add()

	a.tabs.Jump(3)
	assert.Equal(t, 2, a.tabs.active)
	a.tabs.Jump(9)
	assert.Equal(t, 2, a.tabs.active, "out-of-range jump is a no-op")
	a.tabs.Jump(1)
	assert.Equal(t, 0, a.tabs.active)
}

func TestTabsResetToActive(t *testing.T) {
	a := newTabsApp(t)
	a.tabs.add()
	a.tabs.add()
	a.tabs.SwitchTo(1)

	a.tabs.ResetToActive()
	assert.Equal(t, 1, a.tabs.Count())
	assert.Equal(t, 0, a.tabs.active)
	assert.Same(t, a.tabs.tabs[0].stack, a.Content)
}

func TestTabsBarVisibility(t *testing.T) {
	a := newTabsApp(t)
	assert.False(t, a.tabs.barVisible, "bar hidden with a single tab")

	a.tabs.add()
	a.tabs.render()
	assert.True(t, a.tabs.barVisible, "bar shown with 2+ tabs")

	a.tabs.Close(1)
	assert.False(t, a.tabs.barVisible, "bar hidden again after closing back to one tab")
}

func TestTabsTitleTracksStack(t *testing.T) {
	a := newTabsApp(t)
	assert.Equal(t, "new", a.tabs.Current().title())

	a.Content.Push(&tabComp{name: "namespaces"})
	assert.Equal(t, "namespaces", a.tabs.Current().title())

	a.Content.Push(&tabComp{name: "pods", ci: cmd.NewInterpreter("v1/pods ns-1")})
	assert.Equal(t, "pods ns-1", a.tabs.Current().title(), "label comes from the command when present")

	a.Content.Pop()
	assert.Equal(t, "namespaces", a.tabs.Current().title(), "title follows the stack back down")
}

func TestCmdLabel(t *testing.T) {
	uu := map[string]struct {
		in string
		e  string
	}{
		"nil":     {"", ""},
		"gvr":     {"v1/pods", "pods"},
		"gvr+ns":  {"v1/pods ns-1", "pods ns-1"},
		"grouped": {"apps/v1/deployments", "deployments"},
		"short":   {"ns", "ns"},
		"ctx":     {"v1/pods ns-1 @dev", "pods ns-1"},
	}
	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			var ci *cmd.Interpreter
			if u.in != "" {
				ci = cmd.NewInterpreter(u.in)
			}
			assert.Equal(t, u.e, cmdLabel(ci))
		})
	}
}

// ----------------------------------------------------------------------------
// Helpers...

type tabComp struct {
	name             string
	ci               *cmd.Interpreter
	started, stopped int
}

func (c *tabComp) GetCommand() *cmd.Interpreter { return c.ci }
func (*tabComp) SetCommand(*cmd.Interpreter)    {}
func (*tabComp) InCmdMode() bool                { return false }
func (*tabComp) HasFocus() bool                 { return true }
func (*tabComp) Hints() model.MenuHints         { return nil }
func (*tabComp) ExtraHints() map[string]string  { return nil }
func (c *tabComp) Name() string                 { return c.name }
func (*tabComp) Draw(tcell.Screen)              {}
func (*tabComp) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return nil
}
func (*tabComp) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return nil
}
func (*tabComp) SetRect(int, int, int, int)             {}
func (*tabComp) GetRect() (a, b, c, d int)              { return 0, 0, 0, 0 }
func (c *tabComp) GetFocusable() tview.Focusable        { return c }
func (*tabComp) Focus(func(tview.Primitive))            {}
func (*tabComp) Blur()                                  {}
func (c *tabComp) Start()                               { c.started++ }
func (c *tabComp) Stop()                                { c.stopped++ }
func (*tabComp) Init(context.Context) error             { return nil }
func (*tabComp) SetFilter(string, bool)                 {}
func (*tabComp) SetLabelSelector(labels.Selector, bool) {}
