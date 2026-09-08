// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/derailed/k9s/internal"
	"github.com/derailed/k9s/internal/model"
	"github.com/derailed/k9s/internal/slogs"
	"github.com/derailed/k9s/internal/view/cmd"
	"github.com/derailed/tview"
)

// commandGetter reports the command that produced a view.
type commandGetter interface {
	GetCommand() *cmd.Interpreter
}

const (
	// maxTabs caps the number of open tabs.
	maxTabs = 9
	// tabBarHeight is the height of the tab strip when visible.
	tabBarHeight = 1
)

// Tab represents a tab: an independent page stack.
type Tab struct {
	stack *PageStack
	id    string
}

// title returns a display label for the tab's current view.
func (t *Tab) title() string {
	top := t.stack.Top()
	if top == nil {
		return "new"
	}
	if c, ok := top.(commandGetter); ok {
		if s := cmdLabel(c.GetCommand()); s != "" {
			return s
		}
	}
	return top.Name()
}

// cmdLabel renders a compact "resource ns" label from a command.
func cmdLabel(ci *cmd.Interpreter) string {
	if ci == nil {
		return ""
	}
	name := ci.Cmd()
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if ns, ok := ci.NSArg(); ok && ns != "" {
		return name + " " + ns
	}
	return name
}

// Tabs manages a set of page stacks and renders a tab strip above the active one.
type Tabs struct {
	*tview.Flex

	app        *App
	bar        *TabBar
	body       *tview.Pages
	tabs       []*Tab
	active     int
	seq        int
	barVisible bool
}

// NewTabs returns a new tabs manager wrapping the app's current Content.
func NewTabs(app *App) *Tabs {
	t := Tabs{
		Flex: tview.NewFlex().SetDirection(tview.FlexRow),
		app:  app,
		bar:  NewTabBar(app.Styles),
		body: tview.NewPages(),
	}

	first := &Tab{stack: app.Content, id: t.nextID()}
	t.tabs = append(t.tabs, first)
	t.body.AddPage(first.id, first.stack, true, true)

	t.AddItem(t.bar, 0, 0, false)
	t.AddItem(t.body, 0, 1, true)

	return &t
}

// Init initializes the first tab's stack.
func (t *Tabs) Init(ctx context.Context) error {
	if err := t.tabs[0].stack.Init(ctx); err != nil {
		return err
	}
	t.attachChrome(t.tabs[0].stack)
	t.render()

	return nil
}

func (t *Tabs) nextID() string {
	id := fmt.Sprintf("tab-%d", t.seq)
	t.seq++
	return id
}

func (t *Tabs) newCtx() context.Context {
	return context.WithValue(context.Background(), internal.KeyApp, t.app)
}

// attachChrome wires the shared breadcrumbs and menu to the active stack.
func (t *Tabs) attachChrome(ps *PageStack) {
	ps.AddListener(t.app.Crumbs())
	ps.AddListener(t.app.Menu())
	ps.AddListener(t)
}

func (t *Tabs) detachChrome(ps *PageStack) {
	ps.RemoveListener(t.app.Crumbs())
	ps.RemoveListener(t.app.Menu())
	ps.RemoveListener(t)
}

// Stack Protocol...

// StackPushed refreshes the tab strip.
func (t *Tabs) StackPushed(model.Component) { t.render() }

// StackPopped refreshes the tab strip.
func (t *Tabs) StackPopped(_, _ model.Component) { t.render() }

// StackTop refreshes the tab strip.
func (t *Tabs) StackTop(model.Component) { t.render() }

// Count returns the number of open tabs.
func (t *Tabs) Count() int { return len(t.tabs) }

// Current returns the active tab.
func (t *Tabs) Current() *Tab {
	if t.active < 0 || t.active >= len(t.tabs) {
		return nil
	}
	return t.tabs[t.active]
}

// CurrentStack returns the active tab's page stack.
func (t *Tabs) CurrentStack() *PageStack {
	if c := t.Current(); c != nil {
		return c.stack
	}
	return nil
}

// AddAndPrompt opens a new empty tab.
func (t *Tabs) AddAndPrompt() {
	if len(t.tabs) >= maxTabs {
		t.app.Flash().Warnf("Maximum of %d tabs reached", maxTabs)
		return
	}
	t.add()
	t.SwitchTo(len(t.tabs) - 1)
}

// add creates and registers a new tab without switching to it.
func (t *Tabs) add() *Tab {
	tab := &Tab{stack: NewPageStack(), id: t.nextID()}
	if err := tab.stack.Init(t.newCtx()); err != nil {
		slog.Error("Tab stack init failed", slogs.Error, err)
	}
	t.body.AddPage(tab.id, tab.stack, true, false)
	t.tabs = append(t.tabs, tab)

	return tab
}

// SwitchTo activates the tab at index i.
func (t *Tabs) SwitchTo(i int) {
	if i < 0 || i >= len(t.tabs) || i == t.active {
		return
	}
	if t.app.Content.IsTopDialog() {
		t.app.Flash().Warn("Dismiss the dialog before switching tabs")
		return
	}

	if out := t.app.Content; out != nil {
		if top := out.Top(); top != nil {
			top.Stop()
		}
		t.detachChrome(out)
	}

	t.active = i
	in := t.tabs[i].stack
	t.app.Content = in
	t.body.SwitchToPage(t.tabs[i].id)
	t.attachChrome(in)

	t.app.Crumbs().Reset(in.Peek())
	if top := in.Top(); top != nil {
		top.Start()
		t.app.Menu().HydrateMenu(top.Hints())
		t.app.SetFocus(top)
	} else {
		// Empty tab: hand focus to the prompt.
		t.app.Menu().Clear()
		t.app.ResetPrompt(t.app.CmdBuff())
		t.app.CmdBuff().ClearText(true)
	}
	t.render()
}

// Next activates the next tab, wrapping around.
func (t *Tabs) Next() {
	if len(t.tabs) > 1 {
		t.SwitchTo((t.active + 1) % len(t.tabs))
	}
}

// Prev activates the previous tab, wrapping around.
func (t *Tabs) Prev() {
	if len(t.tabs) > 1 {
		t.SwitchTo((t.active - 1 + len(t.tabs)) % len(t.tabs))
	}
}

// Jump activates tab number n (1-based).
func (t *Tabs) Jump(n int) {
	t.SwitchTo(n - 1)
}

// Close removes the tab at index i.
func (t *Tabs) Close(i int) {
	if i < 0 || i >= len(t.tabs) {
		return
	}
	if len(t.tabs) <= 1 {
		t.app.Flash().Warn("Can't close the last tab")
		return
	}
	if t.app.Content.IsTopDialog() {
		t.app.Flash().Warn("Dismiss the dialog before closing the tab")
		return
	}

	wasActive := i == t.active
	victim := t.tabs[i]
	t.detachChrome(victim.stack)
	victim.stack.Dispose()
	t.body.RemovePage(victim.id)
	t.tabs = slices.Delete(t.tabs, i, i+1)

	switch {
	case wasActive:
		target := i
		if target >= len(t.tabs) {
			target = len(t.tabs) - 1
		}
		t.active = -1
		t.SwitchTo(target)
	case i < t.active:
		t.active--
		t.render()
	default:
		t.render()
	}
}

// CloseCurrent closes the active tab.
func (t *Tabs) CloseCurrent() { t.Close(t.active) }

// ResetToActive closes every tab but the active one.
func (t *Tabs) ResetToActive() {
	for i := len(t.tabs) - 1; i >= 0; i-- {
		if i == t.active {
			continue
		}
		tab := t.tabs[i]
		t.detachChrome(tab.stack)
		tab.stack.Dispose()
		t.body.RemovePage(tab.id)
		t.tabs = slices.Delete(t.tabs, i, i+1)
	}
	t.active = 0
	t.render()
}

func (t *Tabs) render() {
	titles := make([]string, len(t.tabs))
	for i, tab := range t.tabs {
		titles[i] = tab.title()
	}
	t.bar.Refresh(titles, t.active)

	t.barVisible = len(t.tabs) > 1
	h := 0
	if t.barVisible {
		h = tabBarHeight
	}
	t.ResizeItem(t.bar, h, 0)
}
