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

// whereMotionIsWanted splits the stylesheet into the rules for someone who has
// not asked their computer for less motion, and all the rest.
func whereMotionIsWanted(t *testing.T, css string) (wanted, always string) {
	t.Helper()
	const open = "@media (prefers-reduced-motion:no-preference){"
	var in, out strings.Builder
	for {
		at := strings.Index(css, open)
		if at < 0 {
			break
		}
		out.WriteString(css[:at])
		end := at + len(open)
		for depth := 1; depth > 0; end++ {
			require.Less(t, end, len(css), "a block that never closes")
			switch css[end] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		in.WriteString(css[at+len(open) : end-1])
		css = css[end:]
	}
	out.WriteString(css)
	require.NotEmpty(t, in.String())
	return in.String(), out.String()
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
// the whole site and nothing drifts. Something that leaves is also kept drawn
// for the time its exit takes. A name the stylesheet uses must be one it sets:
// a browser drops the whole declaration for a name that is not there, and the
// change then happens with no motion at all.
func TestMotion_EveryTransitionIsOnTheScale(t *testing.T) {
	css := builtStylesheet(t)
	onTheScale := regexp.MustCompile(`^[a-z-]+ var\(--t-[a-z]+\) (ease-out|linear|var\(--ease-[a-z]+\)|var\(--spring\))$`)
	keptDrawn := regexp.MustCompile(`^(display|overlay) var\(--t-exit\) allow-discrete$`)
	found := transitions(css)
	require.Greater(t, len(found), 15, "the stylesheet's transitions")
	for _, one := range found {
		if one == "none" || one == "none!important" || slices.Contains(pollGlides, one) {
			continue
		}
		assert.True(t, onTheScale.MatchString(one) || keptDrawn.MatchString(one), "%q is not on the scale", one)
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
		// allow-discrete moves nothing: it keeps what leaves drawn until its exit is over.
		if property == "none" || strings.HasSuffix(one, " allow-discrete") {
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

// TestMotion_LargerMovesTakeLonger: something coming in takes longer than a
// chip changing, a whole page longer than that, and nothing is slow enough to
// wait for.
func TestMotion_LargerMovesTakeLonger(t *testing.T) {
	css := builtStylesheet(t)
	state, enter, page, move := motionTime(t, css, "--t-state"), motionTime(t, css, "--t-enter"), motionTime(t, css, "--t-page"), motionTime(t, css, "--t-move")
	assert.Less(t, state, enter)
	assert.Less(t, motionTime(t, css, "--t-exit"), enter, "what leaves is out of the way sooner than it came")
	assert.Less(t, enter, page)
	assert.LessOrEqual(t, page, 500, "a page change is made all day")
	assert.LessOrEqual(t, move, 700)
}

// TestMotion_TheSpringDoesNotBounce: what turns or lands comes to rest without
// passing its place and coming back.
func TestMotion_TheSpringDoesNotBounce(t *testing.T) {
	css := builtStylesheet(t)
	spring := regexp.MustCompile(`--spring:linear\(([^)]*)\)`).FindStringSubmatch(css)
	require.NotNil(t, spring)
	points := strings.Split(spring[1], ",")
	require.Greater(t, len(points), 8)
	last := 0.0
	for _, point := range points {
		at, err := strconv.ParseFloat(strings.TrimSpace(point), 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, at, last, "it never turns back")
		assert.LessOrEqual(t, at, 1.0, "it never passes where it is going")
		last = at
	}
	assert.Equal(t, 1.0, last)
	assert.Contains(t, css, "--spring:var(--ease-enter)", "a browser without linear() gets the entrance curve")

	// The map's vehicle turns with it, and so does the arrow of a list that folds.
	assert.Contains(t, ruleBody(t, css, ".veh-marker svg"), "transition:transform var(--t-move) var(--spring)")
	assert.Contains(t, ruleBody(t, css, "button.list-group .icon"), "transition:transform var(--t-move) var(--spring)")
}

// TestMotion_APageChangeIsACrossFadeWithARise: the page that leaves fades, the
// one that comes fades in and rises into place, and the menu and the bar on
// top stay where they are. Nothing slides sideways.
func TestMotion_APageChangeIsACrossFadeWithARise(t *testing.T) {
	css := builtStylesheet(t)
	wanted, always := whereMotionIsWanted(t, css)

	assert.Contains(t, wanted, "@view-transition{navigation:auto}")
	assert.NotContains(t, always, "view-transition", "a page just loads for someone who asked for less motion")
	for _, kept := range []string{"rail", "topbar", "tabbar", "page"} {
		assert.Contains(t, wanted, "."+kept+"{view-transition-name:"+kept+"}")
	}
	assert.Contains(t, wanted, "::view-transition-old(page){animation:calc(var(--t-page) * .45) linear both vt-fade-out}")
	assert.Contains(t, wanted, "::view-transition-new(page){animation:calc(var(--t-page) * .6) linear calc(var(--t-page) * .2) both vt-fade-in, var(--t-page) var(--ease-enter) both vt-rise}")
	assert.Contains(t, css, "@keyframes vt-rise{0%{transform:translateY(12px)}}")

	frames := regexp.MustCompile(`@keyframes vt-[a-z-]+\{(?:[^{}]*\{[^{}]*\})*\}`).FindAllString(css, -1)
	require.Len(t, frames, 3, "the fade out, the fade in and the rise")
	for _, frame := range frames {
		assert.NotContains(t, frame, "translateX", "%s moves the page sideways", frame)
	}
}

// TestMotion_ThingsComeInOnOneCurve: a card, a dialog, the search, the account
// menu and the rows of a list all arrive in the same time on the same curve,
// and a list's rows follow each other 40 ms apart, to at most 200 ms.
func TestMotion_ThingsComeInOnOneCurve(t *testing.T) {
	wanted, _ := whereMotionIsWanted(t, builtStylesheet(t))
	const arrive = "transition:opacity var(--t-enter) var(--ease-enter), transform var(--t-enter) var(--ease-enter)"
	for _, rule := range []string{
		".rise>*{animation:rise-in var(--t-enter) var(--ease-enter) both}",
		".float-in{animation:rise-in var(--t-enter) var(--ease-enter) both}",
		".rise>:nth-child(2){animation-delay:40ms}",
		".rise>:nth-child(5){animation-delay:.16s}",
		".rise>:nth-child(n+6){animation-delay:.2s}",
		// What can also leave arrives by a transition, from the starting style.
		"dialog.dialog[open],dialog.palette[open],.map-list,.map-detail{" + arrive + "}",
		".account-menu:popover-open{" + arrive + "}",
		"dialog[open]::backdrop{transition:opacity var(--t-enter) linear}",
		"@starting-style{dialog.dialog[open],dialog.palette[open]{opacity:0;transform:translateY(12px)scale(.98)}dialog[open]::backdrop{opacity:0}" +
			".account-menu:popover-open{opacity:0;transform:translateY(-8px)scale(.98)}.map-list,.map-detail{opacity:0;transform:translateY(12px)}}",
	} {
		assert.Contains(t, wanted, rule)
	}
}

// TestMotion_WhatComesInGoesOut: a message, the vehicle list, the vehicle
// card, a dialog, the search and the account menu fade out where they used to
// vanish. Each is kept drawn while it leaves, and the three drawn over the
// page only where the browser can keep them there.
func TestMotion_WhatComesInGoesOut(t *testing.T) {
	css := builtStylesheet(t)
	wanted, always := whereMotionIsWanted(t, css)
	for _, lingers := range []string{"allow-discrete", "--t-exit)", "@starting-style"} {
		assert.NotContains(t, always, lingers, "nothing arrives or lingers for someone who asked for less motion")
	}

	const leave = "transition:opacity var(--t-exit) var(--ease-exit), transform var(--t-exit) var(--ease-exit)"
	onTop := regexp.MustCompile(`@supports \(overlay:auto\)\{((?:[^{}]*\{[^{}]*\})*)\}`).FindStringSubmatch(wanted)
	require.NotNil(t, onTop)
	inPlace := strings.Replace(wanted, onTop[0], "", 1)

	// Hidden with the hidden attribute, which the stylesheet's own [hidden] rule would otherwise end at once.
	assert.Contains(t, inPlace, ".toast[hidden],.map-list[hidden],.map-detail[hidden]{"+leave+", display var(--t-exit) allow-discrete}")
	assert.Contains(t, inPlace, ".toast[hidden]{opacity:0;transform:translate(-50%,8px)}", "a message keeps its place across the page as it sinks")
	assert.Contains(t, inPlace, ".map-list[hidden],.map-detail[hidden]{opacity:0;pointer-events:none;transform:translateY(8px)}")
	assert.NotContains(t, inPlace, "overlay")

	// Drawn over the page: closed by the browser, and kept on top by the overlay property.
	const onTopLeave = leave + ", overlay var(--t-exit) allow-discrete, display var(--t-exit) allow-discrete"
	for _, rule := range []string{
		"dialog.dialog:not([open]),dialog.palette:not([open]){" + onTopLeave + "}",
		".account-menu:not(:popover-open){" + onTopLeave + "}",
		"dialog.dialog:not([open]),dialog.palette:not([open]){opacity:0;transform:translateY(6px)scale(.98)}",
		".account-menu:not(:popover-open){opacity:0;transform:translateY(-6px)scale(.98)}",
		"dialog:not([open])::backdrop{opacity:0;transition:opacity var(--t-exit) linear, overlay var(--t-exit) allow-discrete, display var(--t-exit) allow-discrete}",
	} {
		assert.Contains(t, onTop[1], rule)
	}

	// The scripts hide these; taking one out of the page would leave nothing to fade.
	shell, err := fs.ReadFile(files, "web/static/js/app.js")
	require.NoError(t, err)
	assert.Contains(t, string(shell), "var close = function () { t.hidden = true; };")
	assert.NotContains(t, string(shell), "t.remove()")
	live, err := fs.ReadFile(files, "web/static/js/map.js")
	require.NoError(t, err)
	assert.Contains(t, string(live), `if (!id || !markers[id]) { shell.hidden = true; card = null; return; }`)
	assert.Contains(t, string(live), `function setList(open) { listOpen = open; $("map-list").hidden = !open; $("map-list-open").hidden = open; }`)
}

// TestMotion_NothingThatLeavesIsHeldByKeyframes: Chrome starts no transition
// on a property that a keyframe animation has been holding. So anything that
// fades out must come in by a transition too: with a keyframe entrance it
// vanishes when closed, as it did before.
func TestMotion_NothingThatLeavesIsHeldByKeyframes(t *testing.T) {
	css := builtStylesheet(t)
	leaves := regexp.MustCompile(`dialog|\.account-menu|\.toast|\.map-list|\.map-detail`)
	for _, rule := range regexp.MustCompile(`([^{}]+)\{[^{}]*?animation:([^;}]*)`).FindAllStringSubmatch(css, -1) {
		assert.False(t, leaves.MatchString(rule[1]), "%s plays the keyframes %s", rule[1], rule[2])
	}

	// float-in is a keyframe entrance, for what stays once it is there.
	page, err := fs.ReadFile(files, "web/templates/views/map.html")
	require.NoError(t, err)
	assert.Contains(t, string(page), `<aside class="map-list float js-only" id="map-list"`)
	assert.Contains(t, string(page), `<aside class="map-detail float" id="map-detail"`)
	live, err := fs.ReadFile(files, "web/static/js/map.js")
	require.NoError(t, err)
	assert.NotContains(t, string(live), "float-in")
}

// TestMotion_OnlyWhereMotionIsWanted: someone who asked their computer for
// less motion sees every change and none of the travel. Outside the rules for
// everyone else, these are the only animations: the script plays the first
// two only where motion is wanted, the next is the 1 ms that shows the line
// along the top with no growth, and the last two are switched off by name.
func TestMotion_OnlyWhereMotionIsWanted(t *testing.T) {
	_, always := whereMotionIsWanted(t, builtStylesheet(t))
	played := map[string]bool{}
	for _, rule := range regexp.MustCompile(`([^{}]+)\{[^{}]*?animation:([^;}]*)`).FindAllStringSubmatch(always, -1) {
		if rule[2] != "none" {
			played[rule[1]] = true
		}
	}
	assert.Equal(t, map[string]bool{
		".pulse.beat .arc-1": true, ".pulse.beat .arc-2": true,
		"html.waiting:after": true,
		".skeleton":          true, ".veh-marker.heard .veh-ring": true,
	}, played)
	assert.Regexp(t, `@media \(prefers-reduced-motion:reduce\)\{[^@]*\.veh-marker\.heard \.veh-ring,\.skeleton\{animation:none\}`, always)

	script, err := fs.ReadFile(files, "web/static/js/app.js")
	require.NoError(t, err)
	assert.Contains(t, string(script), `if (!reduce) { pulse.classList.remove("beat"); void pulse.offsetWidth; pulse.classList.add("beat"); }`)
}
