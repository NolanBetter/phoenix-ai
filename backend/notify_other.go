//go:build !windows

package backend

// WindowClass is the window's class name (used on Windows only).
const WindowClass = "PhoenixAIWindow"

func platformNotify(a *App, title, body, target string) error { return nil }

func flashTaskbar() {}
