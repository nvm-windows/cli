package link

import (
	"common/notify"
	"common/registry"
	"common/settings"
	"encoding/binary"
	"fmt"
	"nvm/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func hideNodejsLink(path string) {
	if !strings.EqualFold(filepath.Base(filepath.Clean(path)), ".nodejs") {
		return
	}

	// /L applies the attribute to the link itself, not the target.
	if err := exec.Command("attrib", "+h", "/L", path).Run(); err == nil {
		return
	}

	// Fallback: preserve existing attributes and add hidden.
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}

	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return
	}

	_ = windows.SetFileAttributes(ptr, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
}

// Link creates an activation link at symlink pointing at source.
// By default it tries an NTFS junction, then falls back to a symlink.
// When settings.RequireActivationSymlinks is true (certified OpenSSH /
// Redirection Guard), only a symlink is created.
func Link(source, symlinkPath string) error {
	// Remove whatever already exists at the path (previous junction, symlink, or empty dir).
	if _, err := os.Lstat(symlinkPath); err == nil {
		if err := os.Remove(symlinkPath); err != nil {
			err = fmt.Errorf("failed to remove existing link %s: %w", symlinkPath, err)
			log.Error(err)
			return err
		}
	}

	requireSymlink := settings.Global().RequireActivationSymlinks
	if !requireSymlink {
		if err := NewJunction(source, symlinkPath); err == nil {
			hideNodejsLink(symlinkPath)
			return nil
		}
	}

	return createSymlink(source, symlinkPath)
}

func createSymlink(source, symlinkPath string) error {
	if err := os.Symlink(source, symlinkPath); err != nil {
		val, _, getErr := registry.Get("HKLM/SOFTWARE/Microsoft/Windows/CurrentVersion/AppModelUnlock/AllowDevelopmentWithoutDevLicense")
		if getErr != nil {
			err = fmt.Errorf("failed to get developer mode setting: %w", getErr)
			log.Error(err)
			return err
		}

		no_devmode := false
		if val == nil {
			no_devmode = true
		} else {
			switch vt := val.(type) {
			case uint32, uint64:
				if vt == 0 {
					no_devmode = true
				}
			}
		}

		if no_devmode {
			// Critical setup toast: no collection (reserved nvm.setup).
			notify.Send(settings.AppId, "Help Enabling Symlinks", "Developer Mode is required to use symlinks.",
				notify.Action{Label: "Open Developer Settings", URL: "ms-settings:developers"},
			)
		}

		log.Errorf("permission denied creating symlink (developer mode %v): %v", val, err)
		return fmt.Errorf("permission denied")
	}

	hideNodejsLink(symlinkPath)
	return nil
}

// IsMountPointJunction reports whether path is an NTFS mount-point junction
// (IO_REPARSE_TAG_MOUNT_POINT), not a symlink.
func IsMountPointJunction(path string) bool {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return false
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false
	}

	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	// Microsoft reparse buffer: first DWORD is ReparseTag.
	buf := make([]byte, 16*1024)
	var n uint32
	const fsctlGetReparsePoint = 0x000900A8
	if err := windows.DeviceIoControl(handle, fsctlGetReparsePoint, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return false
	}
	if n < 4 {
		return false
	}
	tag := binary.LittleEndian.Uint32(buf[0:])
	const ioReparseTagMountPoint = 0xA0000003
	return tag == ioReparseTagMountPoint
}

func Unlink(target string) error {
	return os.Remove(target)
}

// createJunction creates an NTFS junction at target pointing to source using
// DeviceIoControl(FSCTL_SET_REPARSE_POINT) — no elevation required.
func NewJunction(source, target string) error {
	absSrc, err := filepath.Abs(source)
	if err != nil {
		return err
	}

	// Remove any existing junction, symlink, or directory at target
	if _, err := os.Lstat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove existing target %s: %w", target, err)
		}
	}

	if err := os.Mkdir(target, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		_ = os.Remove(target)
		return err
	}
	handle, err := windows.CreateFile(
		targetPtr,
		windows.GENERIC_WRITE,
		0, nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		_ = os.Remove(target)
		return err
	}
	defer windows.CloseHandle(handle)

	// Build a REPARSE_DATA_BUFFER for IO_REPARSE_TAG_MOUNT_POINT (junction).
	// Layout: 4B tag | 2B dataLen | 2B reserved | 2B subOff | 2B subLen | 2B printOff | 2B printLen | PathBuffer
	subName := `\??\` + absSrc
	subW := windows.StringToUTF16(subName)
	subW = subW[:len(subW)-1] // strip null terminator
	printW := windows.StringToUTF16(absSrc)
	printW = printW[:len(printW)-1] // strip null terminator
	subLen := uint16(len(subW) * 2)
	printLen := uint16(len(printW) * 2)
	printOff := subLen + 2 // skip subName + its null terminator
	reparseDataLen := uint16(8 + int(printOff) + int(printLen) + 2)

	buf := make([]byte, 8+int(reparseDataLen))
	binary.LittleEndian.PutUint32(buf[0:], 0xA0000003) // IO_REPARSE_TAG_MOUNT_POINT
	binary.LittleEndian.PutUint16(buf[4:], reparseDataLen)
	// buf[6:8] Reserved = 0
	// buf[8:10] SubstituteNameOffset = 0
	binary.LittleEndian.PutUint16(buf[10:], subLen)
	binary.LittleEndian.PutUint16(buf[12:], printOff)
	binary.LittleEndian.PutUint16(buf[14:], printLen)
	off := 16
	for _, w := range subW {
		binary.LittleEndian.PutUint16(buf[off:], w)
		off += 2
	}
	off += 2 // null separator (already zero)
	for _, w := range printW {
		binary.LittleEndian.PutUint16(buf[off:], w)
		off += 2
	}

	const fsctlSetReparsePoint = 0x000900A4
	var n uint32
	err = windows.DeviceIoControl(handle, fsctlSetReparsePoint,
		&buf[0], uint32(len(buf)), nil, 0, &n, nil)
	if err != nil {
		return err
	}

	hideNodejsLink(target)
	return nil
}
