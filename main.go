package main

import (
	"os"
	"path/filepath"
	"runtime"
)

const appTitle = "sing-box-drover"

func main() {
	runtime.LockOSThread()

	enableDPIAwareness()
	initCommonControls()
	if oleInitialize() {
		defer pOleUninitialize.Call()
	}

	flags := parseAppFlags(os.Args[1:])

	if !acquireSingleInstance("SingBoxDrover_SingleInstance_Mutex", flags.has(afRestart), 10000) {
		messageBox("Another instance of this application is already running.", appTitle, mbIconError)
		return
	}

	firstRun := !fileExists(programDir() + optionsFilename)
	var d *drover

	for {
		if firstRun {
			if showSettingsModal(true, "") != idOK {
				return
			}
			firstRun = false
		}

		var err error
		d, err = newDrover(flags)
		if err == nil {
			break
		}

		if showSettingsModal(false, err.Error()) != idOK {
			messageBox(err.Error(), appTitle, mbIconError)
			return
		}
	}

	if d.needsElevation() {
		if launchSelf(flagsToCmdLine(appFlags(1<<afTun|1<<afRestart)), 0, true) {
			d.destroy()
			return
		}
	}

	ui, err := newMainUI(d, flags)
	if err != nil {
		messageBox(err.Error(), appTitle, mbIconError)
		d.destroy()
		return
	}

	d.setDispatch(ui.post)
	ui.initDrover()
	ui.run()
	d.destroy()
}

func programDir() string {
	return includeTrailingPathDelimiter(filepath.Dir(currentExecutablePath()))
}

func enableDPIAwareness() {
	if r, _, _ := pSetProcessDpiAwareCtx.Call(dpiCtxPerMonV2); r != 0 {
		return
	}
	pSetProcessDPIAware.Call()
}

func oleInitialize() bool {
	r, _, _ := pOleInitialize.Call(0)
	return int32(r) >= 0
}
