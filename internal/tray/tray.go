//go:build darwin

// Package tray manages the macOS system tray icon and menu.
package tray

import (
	"github.com/getlantern/systray"
)

// Callbacks holds the functions the tray calls when menu items are clicked.
type Callbacks struct {
	InitiallyRunning   bool
	InitiallyAutostart bool
	OnOpen             func()
	OnStart            func()
	OnStop             func()
	OnAutostart        func(enabled bool)
	OnQuit             func()
}

// Run initialises the system tray and blocks until quit.
// Call this on the main thread (required by macOS).
func Run(cb Callbacks) {
	systray.Run(func() { onReady(cb) }, func() {})
}

var (
	mStatus    *systray.MenuItem
	mOpen      *systray.MenuItem
	mStart     *systray.MenuItem
	mStop      *systray.MenuItem
	mAutostart *systray.MenuItem
	mQuit      *systray.MenuItem
)

func onReady(cb Callbacks) {
	systray.SetTitle("")
	systray.SetTooltip("routatic-proxy")
	setIcon(false) // start with stopped icon

	mStatus    = systray.AddMenuItem("● 已停止", "")
	mStatus.Disable()
	systray.AddSeparator()

	mOpen      = systray.AddMenuItem("打开控制台…", "")
	systray.AddSeparator()

	mStart      = systray.AddMenuItem("启动代理", "")
	mStop       = systray.AddMenuItem("停止代理", "")
	mStop.Hide()
	systray.AddSeparator()

	mAutostart  = systray.AddMenuItemCheckbox("开机自启", "", false)
	systray.AddSeparator()

	mQuit       = systray.AddMenuItem("退出", "")

	// Set initial state safely now that menu items are created
	SetRunning(cb.InitiallyRunning)
	SetAutostart(cb.InitiallyAutostart)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				if cb.OnOpen != nil { cb.OnOpen() }
			case <-mStart.ClickedCh:
				if cb.OnStart != nil { cb.OnStart() }
			case <-mStop.ClickedCh:
				if cb.OnStop != nil { cb.OnStop() }
			case <-mAutostart.ClickedCh:
				checked := !mAutostart.Checked()
				if checked { mAutostart.Check() } else { mAutostart.Uncheck() }
				if cb.OnAutostart != nil { cb.OnAutostart(checked) }
			case <-mQuit.ClickedCh:
				systray.Quit()
				if cb.OnQuit != nil { cb.OnQuit() }
			}
		}
	}()
}

// SetRunning updates the tray menu to reflect proxy running state.
func SetRunning(running bool) {
	if mStatus == nil || mStart == nil || mStop == nil {
		return
	}
	if running {
		setIcon(true)
		mStatus.SetTitle("● 运行中")
		mStart.Hide()
		mStop.Show()
	} else {
		setIcon(false)
		mStatus.SetTitle("● 已停止")
		mStop.Hide()
		mStart.Show()
	}
}

// SetAutostart updates the autostart checkbox state.
func SetAutostart(enabled bool) {
	if mAutostart == nil {
		return
	}
	if enabled {
		mAutostart.Check()
	} else {
		mAutostart.Uncheck()
	}
}

// setIcon sets a minimal text icon (systray title) depending on state.
// A real app would embed an .icns; here we use a unicode bullet.
func setIcon(running bool) {
	if running {
		systray.SetTitle("⚡")
	} else {
		systray.SetTitle("⚡")
	}
}
