package installer

import (
	"bufio"
	"common/settings"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	allowed := normalizeAllowedSigners(settings.Global().AllowedSigners)
	if len(allowed) == 0 {
		return "", fmt.Errorf("no allowed code signers configured")
	}

	signer := signerOrganization(exePath)
	if signer == "" {
		return "", fmt.Errorf("unable to verify code signer for %s", filepath.Base(exePath))
	}

	if !isAllowedSigner(signer, allowed) {
		return signer, fmt.Errorf("code signer %q is not allowed (allowed signers: %s)", signer, strings.Join(allowed, ", "))
	}

	return signer, nil
}

func normalizeAllowedSigners(signers []string) []string {
	normalized := make([]string, 0, len(signers))
	for _, signer := range signers {
		trimmed := strings.TrimSpace(signer)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func isAllowedSigner(signer string, allowed []string) bool {
	candidate := strings.TrimSpace(signer)
	if candidate == "" {
		return false
	}
	for _, allowedSigner := range allowed {
		if strings.EqualFold(candidate, strings.TrimSpace(allowedSigner)) {
			return true
		}
	}
	return false
}

func nodePublisher(exePath string) string {
	const fallback = "OpenJS Foundation"
	if name := signerOrganization(exePath); name != "" {
		return name
	}
	return peCompanyName(exePath, fallback)
}

func signerOrganization(exePath string) string {
	exeW, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return ""
	}

	crypt32dll := windows.NewLazySystemDLL("crypt32.dll")
	cryptMsgGetParam := crypt32dll.NewProc("CryptMsgGetParam")
	cryptMsgClose := crypt32dll.NewProc("CryptMsgClose")

	var certStore, msg windows.Handle
	err = windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(exeW),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0, nil, nil, nil,
		&certStore, &msg, nil,
	)
	if err != nil {
		return ""
	}
	defer windows.CertCloseStore(certStore, 0)
	defer cryptMsgClose.Call(uintptr(msg))

	const cmsgSignerCertInfoParam = 7
	var size uint32
	r, _, _ := cryptMsgGetParam.Call(uintptr(msg), cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	buf := make([]byte, size)
	r, _, _ = cryptMsgGetParam.Call(uintptr(msg), cmsgSignerCertInfoParam, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return ""
	}

	signerCertInfo := (*windows.CertInfo)(unsafe.Pointer(&buf[0]))
	cert, _ := windows.CertFindCertificateInStore(
		certStore,
		windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(signerCertInfo), nil,
	)
	runtime.KeepAlive(buf)
	if cert == nil {
		return ""
	}

	n := windows.CertGetNameString(cert, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, nil, 0)
	if n == 0 {
		return ""
	}
	nameBuf := make([]uint16, n)
	windows.CertGetNameString(cert, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, &nameBuf[0], n)
	return strings.TrimSpace(windows.UTF16ToString(nameBuf))
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
