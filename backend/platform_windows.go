//go:build windows

package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createNoWindow         = 0x08000000
	createNewProcessGroup  = 0x00000200
	createBreakawayFromJob = 0x01000000
)

// prepareCmd stops the engine from flashing a console window.
func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// launchInstaller runs a downloaded Phoenix installer silently (/S). With
// relaunch, the installer opens Phoenix again when it's done (/UPDATE).
// It's started on its own so it keeps running after Phoenix closes.
func launchInstaller(path string, relaunch bool) error {
	args := []string{"/S"}
	if relaunch {
		args = append(args, "/UPDATE")
	}
	start := func(flags uint32) (*exec.Cmd, error) {
		cmd := exec.Command(path, args...)
		cmd.Dir = filepath.Dir(path)
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
		return cmd, cmd.Start()
	}
	cmd, err := start(createNewProcessGroup | createBreakawayFromJob)
	if err != nil {
		// Some launchers don't allow leaving their job; try without.
		if cmd, err = start(createNewProcessGroup); err != nil {
			return err
		}
	}
	return cmd.Process.Release()
}

var (
	jobOnce   sync.Once
	jobHandle windows.Handle
)

// bindToApp puts the engine in a Windows "job" that closes it automatically
// if Phoenix AI exits or crashes, so it never keeps using memory in the background.
func bindToApp(p *os.Process) {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		if err != nil {
			windows.CloseHandle(h)
			return
		}
		jobHandle = h
	})
	if jobHandle == 0 || p == nil {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph)
	_ = windows.AssignProcessToJobObject(jobHandle, ph)
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var procGlobalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func totalRAM() int64 {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0
	}
	return int64(m.TotalPhys)
}

func diskFree(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0
	}
	return int64(free)
}

const engineExeName = "llama-server.exe"
