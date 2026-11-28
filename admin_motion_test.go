package main

import (
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin's motion comes from one short scale of times and curves, set once
// in the stylesheet. These tests read the built stylesheet, which is the file a
// browser gets.

func builtStylesheet(t *testing.T) string {
	t.Helper()
	css, err := fs.ReadFile(files, "web/static/css/admin.css")
	require.NoError(t, err)
	return string(css)
}

// motionTime is the time a custom property such as --t-hover is set to.
func motionTime(t *testing.T, css, name string) int {
	t.Helper()
	set := regexp.MustCompile(regexp.QuoteMeta(name) + `:([0-9.]+)(ms|s)[;}]`).FindStringSubmatch(css)
	require.NotNil(t, set, "%s is not set", name)
	n, err := strconv.ParseFloat(set[1], 64)
	require.NoError(t, err)
	if set[2] == "s" {
		n *= 1000
	}
	return int(n + 0.5)
}

// ruleBody is what the rule with exactly this selector sets.
func ruleBody(t *testing.T, css, selector string) string {
	t.Helper()
	rule := regexp.MustCompile(`[{}]` + regexp.QuoteMeta(selector) + `\{([^}]*)\}`).FindStringSubmatch(css)
	require.NotNil(t, rule, "no rule for %s", selector)
	return rule[1]
}

// splitOutsideBrackets splits a value at the commas that are not inside
// brackets, so the commas of a cubic-bezier() stay with it.
func splitOutsideBrackets(value string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	return append(parts, strings.TrimSpace(value[start:]))
}

// transitions returns every transition the stylesheet sets, one per property.
func transitions(css string) []string {
	var all []string
	for _, set := range regexp.MustCompile(`[{;]transition:([^;}]*)`).FindAllStringSubmatch(css, -1) {
		all = append(all, splitOutsideBrackets(set[1])...)
	}
	return all
}

// The two glides between polls, which take as long as a poll.
var pollGlides = []string{"left 10s linear", "transform 10s linear"}

// TestMotion_EveryTransitionIsOnTheScale: a transition takes its time from the
// scale and its curve from the scale or a plain keyword, so one edit retimes
// the whole site and nothing drifts. A name the stylesheet uses must be one it
// sets: a browser drops the whole declaration for a name that is not there,
// and the change then happens with no motion at all.
func TestMotion_EveryTransitionIsOnTheScale(t *testing.T) {
	css := builtStylesheet(t)
	onTheScale := regexp.MustCompile(`^[a-z-]+ var\(--t-[a-z]+\) (ease-out|linear|var\(--ease-[a-z]+\))$`)
	found := transitions(css)
	require.Greater(t, len(found), 15, "the stylesheet's transitions")
	for _, one := range found {
		if one == "none" || one == "none!important" || slices.Contains(pollGlides, one) {
			continue
		}
		assert.Regexp(t, onTheScale, one)
	}

	set := map[string]bool{}
	for _, name := range regexp.MustCompile(`(--(?:t|ease)-[a-z]+|--spring[a-z-]*):`).FindAllStringSubmatch(css, -1) {
		set[name[1]] = true
	}
	used := regexp.MustCompile(`var\((--(?:t|ease)-[a-z]+|--spring[a-z-]*)\)`).FindAllStringSubmatch(css, -1)
	require.NotEmpty(t, used)
	for _, name := range used {
		assert.True(t, set[name[1]], "%s is used but never set", name[1])
	}
}

// TestMotion_FrequentChangesStayQuick: a hover or a press happens hundreds of
// times a day and a chip or a toggle dozens. Slower than this and the site
// feels heavy however smooth each one is.
func TestMotion_FrequentChangesStayQuick(t *testing.T) {
	css := builtStylesheet(t)
	hover, state := motionTime(t, css, "--t-hover"), motionTime(t, css, "--t-state")
	assert.LessOrEqual(t, hover, 150, "hover and press")
	assert.LessOrEqual(t, state, 250, "a chip, a toggle, a tooltip")
	assert.Less(t, hover, state, "the smaller change is the shorter one")

	// A chip and a toggle change the same way, and a hover never takes the longer time.
	for _, selector := range []string{".chip", ".seg>*"} {
		assert.Contains(t, ruleBody(t, css, selector), "transition:background-color var(--t-state) var(--ease-enter), color var(--t-state) var(--ease-enter), box-shadow var(--t-state) var(--ease-enter)", selector)
	}
	for _, selector := range []string{".btn", ".rail-item", ".icon-btn", ".table tbody tr"} {
		body := ruleBody(t, css, selector)
		assert.Contains(t, body, "transition:background-color var(--t-hover) ease-out", selector)
		assert.NotContains(t, body, "--t-state", selector)
	}
}

// A browser can run a change of transform or opacity without laying the page
// out again, and a change of colour or shadow with a repaint alone. Anything
// else that moves lays the page out on every frame.
var paintOnly = map[string]bool{
	"opacity": true, "transform": true,
	"background-color": true, "background": true, "color": true, "border-color": true, "box-shadow": true, "stroke-dashoffset": true,
}

// The two places that do move a laid-out property, both small and both there
// before the scale: the vehicle on a route's straight line, and the vehicle in
// the welcome picture.
var layoutMoves = map[string]bool{"transition left": true, "keyframes pic-glide top": true}

// TestMotion_NothingElseAnimatesLayout: every transition and every keyframe
// changes only what a browser can animate without laying the page out again.
func TestMotion_NothingElseAnimatesLayout(t *testing.T) {
	css := builtStylesheet(t)
	seen := map[string]bool{}
	for _, one := range transitions(css) {
		property := strings.Fields(strings.TrimSuffix(one, "!important"))[0]
		if property == "none" {
			continue
		}
		if !paintOnly[property] {
			seen["transition "+property] = true
		}
	}
	frames := regexp.MustCompile(`@keyframes ([a-z0-9-]+)\{((?:[^{}]*\{[^{}]*\})*)\}`).FindAllStringSubmatch(css, -1)
	require.Greater(t, len(frames), 15, "the stylesheet's keyframes")
	for _, frame := range frames {
		for _, step := range regexp.MustCompile(`\{([^{}]*)\}`).FindAllStringSubmatch(frame[2], -1) {
			for _, declaration := range strings.Split(step[1], ";") {
				property, _, found := strings.Cut(declaration, ":")
				if found && !paintOnly[property] {
					seen["keyframes "+frame[1]+" "+property] = true
				}
			}
		}
	}
	assert.Equal(t, layoutMoves, seen)
}
