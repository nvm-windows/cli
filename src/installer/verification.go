package installer

import (
	"bufio"
	"common/settings"
	"common/verify"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func verifyNodeSHASUM(file, shasum string) (bool, error) {
	if file == "" {
		return false, fmt.Errorf("missing file path")
	}
	if shasum == "" {
		return false, fmt.Errorf("missing SHASUM")
	}

	f, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return false, err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	target := filepath.Base(file)

	shasums, err := os.Open(shasum)
	if err != nil {
		return false, err
	}
	defer shasums.Close()

	scanner := bufio.NewScanner(shasums)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		expected := strings.TrimLeft(strings.ToLower(parts[0]), "*")
		name := filepath.Base(strings.TrimSpace(parts[1]))
		if name != target {
			continue
		}
		return strings.EqualFold(actual, expected), nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, fmt.Errorf("SHASUM entry not found for %s", target)
}

func verifyAllowedSigner(exePath string) (string, error) {
	// WinVerifyTrust first, then AllowedSigners vendor policy (settings.AllowedSigners).
	return verify.VerifyNodeExecutable(exePath, settings.Global().AllowedSigners)
}

func nodePublisher(exePath string) string {
	const fallback = "OpenJS Foundation"
	if name := verify.SignerOrganization(exePath); name != "" {
		return name
	}
	return peCompanyName(exePath, fallback)
}

func peCompanyName(exePath, fallback string) string {
	vdll := windows.NewLazySystemDLL("version.dll")
	getInfoSize := vdll.NewProc("GetFileVersionInfoSizeW")
	getInfo := vdll.NewProc("GetFileVersionInfoW")
	queryValue := vdll.NewProc("VerQueryValueW")

	path, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return fallback
	}

	size, _, _ := getInfoSize.Call(uintptr(unsafe.Pointer(path)), 0)
	if size == 0 {
		return fallback
	}

	buf := make([]byte, size)
	ret, _, _ := getInfo.Call(uintptr(unsafe.Pointer(path)), 0, size, uintptr(unsafe.Pointer(&buf[0])))
	if ret == 0 {
		return fallback
	}

	var lang, codepage uint16
	transBlock, _ := windows.UTF16PtrFromString(`\VarFileInfo\Translation`)
	var transPtr uintptr
	var transLen uint32
	ret, _, _ = queryValue.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(transBlock)), uintptr(unsafe.Pointer(&transPtr)), uintptr(unsafe.Pointer(&transLen)))
	if ret != 0 && transLen >= 4 {
		lang = *(*uint16)(unsafe.Pointer(transPtr))
		codepage = *(*uint16)(unsafe.Pointer(transPtr + 2))
	} else {
		lang, codepage = 0x0409, 0x04B0
	}

	subkey, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\StringFileInfo\%04X%04X\CompanyName`, lang, codepage))
	var companyPtr uintptr
	var companyLen uint32
	ret, _, _ = queryValue.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(subkey)), uintptr(unsafe.Pointer(&companyPtr)), uintptr(unsafe.Pointer(&companyLen)))
	if ret == 0 || companyLen == 0 {
		return fallback
	}

	company := strings.TrimSpace(windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(companyPtr)), companyLen)))
	if company == "" {
		return fallback
	}
	return company
}
