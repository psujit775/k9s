// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"fmt"
	"strings"

	"github.com/derailed/k9s/internal/config"
	"github.com/derailed/k9s/internal/ui"
	"github.com/derailed/tview"
)

// maxTabTitle caps the tab label length.
const maxTabTitle = 20

// TabBar renders the strip of open tabs.
type TabBar struct {
	*tview.TextView

	styles *config.Styles
	titles []string
	active int
}

// NewTabBar returns a new tab bar.
func NewTabBar(styles *config.Styles) *TabBar {
	b := TabBar{
		styles:   styles,
		TextView: tview.NewTextView(),
	}
	b.SetBackgroundColor(styles.BgColor())
	b.SetTextAlign(tview.AlignLeft)
	b.SetBorderPadding(0, 0, 1, 1)
	b.SetDynamicColors(true)
	styles.AddListener(&b)

	return &b
}

// StylesChanged notifies skin changed.
func (b *TabBar) StylesChanged(s *config.Styles) {
	b.styles = s
	b.SetBackgroundColor(s.BgColor())
	b.refresh()
}

// Refresh updates the bar with new tab titles.
func (b *TabBar) Refresh(titles []string, active int) {
	b.titles, b.active = titles, active
	b.refresh()
}

func (b *TabBar) refresh() {
	b.Clear()
	crumb := b.styles.Frame().Crumb
	bodyBg := b.styles.Body().BgColor
	for i, title := range b.titles {
		bg := crumb.BgColor
		if i == b.active {
			bg = crumb.ActiveColor
		}
		_, _ = fmt.Fprintf(b, "[%s:%s:b] %d:%s [-:%s:-] ",
			crumb.FgColor,
			bg,
			i+1,
			clampTitle(title),
			bodyBg,
		)
	}
}

func clampTitle(s string) string {
	return ui.Truncate(strings.ToLower(strings.TrimSpace(s)), maxTabTitle)
}
