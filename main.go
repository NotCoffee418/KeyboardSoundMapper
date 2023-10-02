package main

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	"io/fs"
	"syscall"
	"unsafe"
)

//go:embed sounds/*
var embeddedSounds embed.FS
var (
	soundBuffers = map[uint8]*beep.Buffer{}
	mixer        *beep.Mixer
)

func init() {
	mixer = &beep.Mixer{}
	speaker.Init(beep.SampleRate(44100), 44100/10) // You can set this to the sample rate of your WAV files
	speaker.Play(mixer)
}

var (
	moduser32               = syscall.NewLazyDLL("user32.dll")
	modwinmm                = syscall.NewLazyDLL("winmm.dll")
	procSetHook             = moduser32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = moduser32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = moduser32.NewProc("UnhookWindowsHookEx")
	procPlaySound           = modwinmm.NewProc("PlaySoundW")
	procGetMessage          = moduser32.NewProc("GetMessageW")
	procTranslateMessage    = moduser32.NewProc("TranslateMessage")
	procDispatchMessage     = moduser32.NewProc("DispatchMessageW")
)

type MSG struct {
	HWND   uintptr
	UINT   uint32
	WPARAM uintptr
	LPARAM uintptr
	DWORD  uint32
	POINT  struct {
		X, Y int32
	}
}

func main() {
	// Load all sound files into memory
	for i := uint8(1); i <= 5; i++ {
		soundData, err := fs.ReadFile(embeddedSounds, fmt.Sprintf("sounds/p%d.wav", i))
		if err != nil {
			panic(err)
		}

		r := bytes.NewReader(soundData)
		streamer, format, err := wav.Decode(r)
		if err != nil {
			panic(err)
		}

		buffer := beep.NewBuffer(format)
		buffer.Append(streamer)
		streamer.Close()

		soundBuffers[i] = buffer
	}

	fmt.Println("App started")

	// Set up the hook
	hookID := setHook()
	defer unhookWindowsHookEx(hookID)

	// Run the message loop
	var message MSG
	for {
		r1, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if err != nil {
			panic(err)
		}
		if r1 == 0 { // WM_QUIT
			break
		}
		_, _, err = procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		if err != nil {
			panic(err)
		}
		_, _, err = procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
		if err != nil {
			panic(err)
		}
	}
}

func setHook() uintptr {
	hookProc := syscall.NewCallback(hookCallback)
	hookID, _, _ := procSetHook.Call(13, uintptr(hookProc), 0, 0)
	return hookID
}

func unhookWindowsHookEx(hookID uintptr) {
	_, _, err := procUnhookWindowsHookEx.Call(hookID)
	if err != nil {
		panic(err)
	}
}

func hookCallback(nCode int, wparam, lparam uintptr) uintptr {
	// Pass the key event to the next hook
	result, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wparam, lparam)

	if nCode >= 0 && wparam == 0x100 { // WM_KEYDOWN
		go func() {
			vkCode := *(*uint32)(unsafe.Pointer(lparam))
			handleKeyDown(vkCode) //
		}()
	}

	return result
}

func handleKeyDown(vkCode uint32) {
	//fmt.Println("Key down:", vkCode)
	if sound, exists := soundMap[vkCode]; exists {
		playSound(sound)
	}
}

func playSound(soundID uint8) {
	buffer, exists := soundBuffers[soundID]
	if !exists {
		panic(fmt.Sprintf("Sound %d does not exist", soundID))
	}

	streamer := buffer.Streamer(0, buffer.Len())
	mixer.Add(streamer)
	//fmt.Println("Playing sound", soundID)
}
