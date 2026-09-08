// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package view

import (
	"testing"

	"github.com/derailed/k9s/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestTabBarRefresh(t *testing.T) {
	b := NewTabBar(config.NewStyles())
	b.Refresh([]string{"ns", "ing", "pods ns-1"}, 1)

	assert.Equal(t,
		"[#000000:#00ffff:b] 1:ns [-:#000000:-] [#000000:#ffa500:b] 2:ing [-:#000000:-] [#000000:#00ffff:b] 3:pods ns-1 [-:#000000:-] \n",
		b.GetText(false),
	)
}

func TestTabBarEmpty(t *testing.T) {
	b := NewTabBar(config.NewStyles())
	b.Refresh(nil, 0)
	assert.Empty(t, b.GetText(false))
}

func TestTabBarStylesChanged(t *testing.T) {
	b := NewTabBar(config.NewStyles())
	b.Refresh([]string{"ns"}, 0)
	b.StylesChanged(config.NewStyles())
	assert.Contains(t, b.GetText(false), "1:ns")
}
