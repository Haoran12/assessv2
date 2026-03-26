//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	englishKeyboardLayoutID  = "00000409"
	klfActivate              = 0x00000001
	wmInputLangChangeRequest = 0x0050
)

var (
	user32DLL                  = windows.NewLazySystemDLL("user32.dll")
	loadKeyboardLayoutWProc    = user32DLL.NewProc("LoadKeyboardLayoutW")
	activateKeyboardLayoutProc = user32DLL.NewProc("ActivateKeyboardLayout")
	getForegroundWindowProc    = user32DLL.NewProc("GetForegroundWindow")
	postMessageWProc           = user32DLL.NewProc("PostMessageW")
)

func switchToEnglishInputMethod() error {
	layoutIDUTF16, err := windows.UTF16PtrFromString(englishKeyboardLayoutID)
	if err != nil {
		return fmt.Errorf("build english keyboard layout id: %w", err)
	}

	hkl, _, loadErr := loadKeyboardLayoutWProc.Call(
		uintptr(unsafe.Pointer(layoutIDUTF16)),
		uintptr(klfActivate),
	)
	if hkl == 0 {
		return fmt.Errorf("load english keyboard layout: %w", loadErr)
	}

	activateKeyboardLayoutProc.Call(hkl, 0)

	hwnd, _, _ := getForegroundWindowProc.Call()
	if hwnd == 0 {
		return nil
	}

	ok, _, postErr := postMessageWProc.Call(
		hwnd,
		uintptr(wmInputLangChangeRequest),
		0,
		hkl,
	)
	if ok == 0 {
		return fmt.Errorf("request input language change: %w", postErr)
	}

	return nil
}
