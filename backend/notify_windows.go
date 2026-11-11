//go:build windows

package backend

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"git.sr.ht/~jackmordaunt/go-toast/v2/tmpl"
	"git.sr.ht/~jackmordaunt/go-toast/v2/wintoast"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// ToastAppID is the name Windows shows on Phoenix's notifications.
	ToastAppID = "Phoenix AI"
	// ToastGUID identifies Phoenix to Windows when a notification is clicked.
	ToastGUID = "{6E3B1F4A-92C8-4D57-A0B3-7C1E5D9F2A68}"
	// WindowClass is the window's class name, used to flash its taskbar button.
	WindowClass = "PhoenixAIWindow"
)

var (
	toastOnce sync.Once
	toastErr  error
)

func setupToasts(a *App) error {
	toastOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			toastErr = err
			return
		}
		icon := filepath.Join(a.paths.Root, "notification-icon.png")
		_ = os.WriteFile(icon, notificationIcon, 0o644)
		// ActivationExe is left out on purpose: go-toast would register it
		// under its own default class ID, not ours. Phoenix does it below.
		toastErr = toast.SetAppData(toast.AppData{
			AppID:    ToastAppID,
			GUID:     ToastGUID,
			IconPath: icon,
		})
		if toastErr != nil {
			return
		}
		// The program Windows starts when a notification is clicked after
		// Phoenix was closed.
		if k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\CLSID\`+ToastGUID+`\LocalServer32`, registry.SET_VALUE); err == nil {
			_ = k.SetStringValue("", `"`+exe+`"`)
			k.Close()
		}
		toast.SetActivationCallback(func(args string, _ []toast.UserData) {
			a.onNotificationClick(args)
		})
	})
	return toastErr
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func platformNotify(a *App, title, body, target string) error {
	if err := setupToasts(a); err != nil {
		return err
	}
	// The template puts the title and body inside CDATA sections and the
	// click target in an attribute, and escapes neither, so do it here.
	cdata := strings.NewReplacer("]]>", "]] >")
	n := toast.Notification{
		AppID:               ToastAppID,
		Title:               cdata.Replace(title),
		Body:                cdata.Replace(body),
		ActivationType:      toast.Foreground,
		ActivationArguments: xmlEscape(target),
		Audio:               toast.Default,
		Duration:            toast.Short,
	}
	var buf bytes.Buffer
	if err := tmpl.XMLTemplate.Execute(&buf, &n); err != nil {
		return err
	}
	return wintoast.Push(ToastAppID, buf.String())
}

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW   = user32.NewProc("FindWindowW")
	procFlashWindowEx = user32.NewProc("FlashWindowEx")
)

type flashInfo struct {
	Size    uint32
	Hwnd    uintptr
	Flags   uint32
	Count   uint32
	Timeout uint32
}

// flashTaskbar makes Phoenix's taskbar button flash until you switch to it.
func flashTaskbar() {
	cls, err := windows.UTF16PtrFromString(WindowClass)
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), 0)
	if hwnd == 0 {
		return
	}
	const flashTray, flashUntilForeground = 0x2, 0xC
	fi := flashInfo{Hwnd: hwnd, Flags: flashTray | flashUntilForeground}
	fi.Size = uint32(unsafe.Sizeof(fi))
	procFlashWindowEx.Call(uintptr(unsafe.Pointer(&fi)))
}
