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

const vsFixedFileInfoSignature = 0xFEEF04BD

type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

type peCoreVersion struct {
	major uint32
	minor uint32
	patch uint32
}

func parseCoreVersion(raw string) (peCoreVersion, bool) {
	s := strings.TrimSpace(raw)
	if len(s) >= 1 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	if s == "" {
		return peCoreVersion{}, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || parts[0] == "" {
		return peCoreVersion{}, false
	}
	major, err := parseVersionPart(parts[0])
	if err != nil {
		return peCoreVersion{}, false
	}
	minor := uint32(0)
	patch := uint32(0)
	if len(parts) > 1 {
		minor, err = parseVersionPart(parts[1])
		if err != nil {
			return peCoreVersion{}, false
		}
	}
	if len(parts) > 2 {
		patch, err = parseVersionPart(parts[2])
		if err != nil {
			return peCoreVersion{}, false
		}
	}
	return peCoreVersion{major: major, minor: minor, patch: patch}, true
}

func parseVersionPart(raw string) (uint32, error) {
	if raw == "" {
		return 0, fmt.Errorf("empty")
	}
	var n uint64
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + uint64(c-'0')
		if n > 0xffffffff {
			return 0, fmt.Errorf("overflow")
		}
	}
	return uint32(n), nil
}

func coreVersionEqual(file peCoreVersion, expected string) bool {
	folder, ok := parseCoreVersion(expected)
	if !ok {
		return false
	}
	return file.major == folder.major && file.minor == folder.minor && file.patch == folder.patch
}

func peFileVersion(exePath string) (peCoreVersion, error) {
	vdll := windows.NewLazySystemDLL("version.dll")
	getInfoSize := vdll.NewProc("GetFileVersionInfoSizeW")
	getInfo := vdll.NewProc("GetFileVersionInfoW")
	queryValue := vdll.NewProc("VerQueryValueW")

	path, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return peCoreVersion{}, err
	}

	size, _, _ := getInfoSize.Call(uintptr(unsafe.Pointer(path)), 0)
	if size == 0 {
		return peCoreVersion{}, fmt.Errorf("VERSIONINFO missing")
	}

	buf := make([]byte, size)
	ret, _, _ := getInfo.Call(uintptr(unsafe.Pointer(path)), 0, size, uintptr(unsafe.Pointer(&buf[0])))
	if ret == 0 {
		return peCoreVersion{}, fmt.Errorf("VERSIONINFO missing")
	}

	rootBlock, err := windows.UTF16PtrFromString(`\`)
	if err != nil {
		return peCoreVersion{}, err
	}
	var infoPtr uintptr
	var infoLen uint32
	ret, _, _ = queryValue.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(rootBlock)), uintptr(unsafe.Pointer(&infoPtr)), uintptr(unsafe.Pointer(&infoLen)))
	if ret == 0 || infoPtr == 0 || infoLen < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return peCoreVersion{}, fmt.Errorf("VERSIONINFO missing")
	}

	info := (*vsFixedFileInfo)(unsafe.Pointer(infoPtr))
	if info.Signature != vsFixedFileInfoSignature {
		return peCoreVersion{}, fmt.Errorf("VERSIONINFO missing")
	}

	return peCoreVersion{
		major: info.FileVersionMS >> 16,
		minor: info.FileVersionMS & 0xffff,
		patch: info.FileVersionLS >> 16,
	}, nil
}

func verifyNodeBinaryMatchesVersion(versionDir, version string) error {
	exePath := filepath.Join(versionDir, "node.exe")
	if _, err := os.Stat(exePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("NVM4314: unable to inspect %s: %w", exePath, err)
	}

	file, err := peFileVersion(exePath)
	if err != nil {
		return fmt.Errorf("NVM4314: unable to read FileVersion from %s: %w", exePath, err)
	}
	if !coreVersionEqual(file, version) {
		return fmt.Errorf("NVM4314: node.exe FileVersion %d.%d.%d does not match folder v%s", file.major, file.minor, file.patch, version)
	}
	return nil
}
