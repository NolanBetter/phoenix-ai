package backend

import (
	_ "embed"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Windows notifications appear when an answer or download finishes while
// Phoenix isn't the window in front. The taskbar button also flashes.

//go:embed assets/notification-icon.png
var notificationIcon []byte

// showNotification is swapped out in tests.
var showNotification = platformNotify

// SetWindowActive is called by the window when it gains or loses focus.
func (a *App) SetWindowActive(active bool) {
	a.viewMu.Lock()
	a.active = active
	a.viewMu.Unlock()
}

func (a *App) windowActive() bool {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.active
}

// notifyIfAway shows a notification unless Phoenix is in front or the
// user turned notifications off. target says what clicking it opens:
// "chat:<id>" or "models".
func (a *App) notifyIfAway(title, body, target string) {
	if a.windowActive() || !a.getSettings().Notifications {
		return
	}
	go func() {
		_ = showNotification(a, title, body, target)
		flashTaskbar()
	}()
}

// onNotificationClick runs when a notification is clicked.
func (a *App) onNotificationClick(target string) {
	if a.hooks.Focus != nil {
		a.hooks.Focus()
	}
	a.emit("notify:open", target)
}

var markdownNoise = regexp.MustCompile("(?m)^#{1,6}\\s+|[*_`>~]|\\[([^\\]]*)\\]\\([^)]*\\)")

// previewText turns the start of an answer into one plain line.
func previewText(s string, n int) string {
	s = markdownNoise.ReplaceAllString(s, "$1")
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	cut := string(r[:n])
	if i := strings.LastIndex(cut, " "); i > n/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:") + "…"
}
