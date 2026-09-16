package vmlib

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DomainInfo holds parsed libvirt domain details.
type DomainInfo struct {
	Name  string
	UUID  string
	State string
}

func GetDomainInfo(vmName string) (*DomainInfo, error) {
	state, err := exec.Command("virsh", "-c", "qemu:///system", "domstate", vmName).Output()
	if err != nil {
		return nil, fmt.Errorf("virsh domstate: %w", err)
	}

	dom, err := getDomainXML(vmName)
	if err != nil {
		return nil, fmt.Errorf("virsh dumpxml: %w", err)
	}
	info, err := parseDomainXML(dom)
	if err != nil {
		return nil, err
	}
	info.State = extractState(string(state))
	return info, nil
}

func getDomainXML(vmName string) ([]byte, error) {
	out, err := exec.Command("virsh", "-c", "qemu:///system", "dumpxml", vmName).Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

func parseDomainXML(xmlData []byte) (*DomainInfo, error) {
	var domain struct {
		Name string `xml:"name"`
		UUID string `xml:"uuid"`
	}
	if err := xml.Unmarshal(xmlData, &domain); err != nil {
		return nil, fmt.Errorf("xml parse: %w", err)
	}
	return &DomainInfo{
		Name:  domain.Name,
		UUID:  domain.UUID,
		State: "",
	}, nil
}

func extractState(raw string) string {
	return strings.TrimSpace(raw)
}

func ListGuestApps(vmName string, vmCfg VMConfig) ([]string, error) {
	sshTarget := resolveSSHTarget(vmName, vmCfg)
	// Get user from sshTarget for home dir
	user := sshTarget[:strings.Index(sshTarget, "@")]

	script := "find /usr/share/applications /home/" + user + "/.local/share/applications /var/lib/flatpak/exports/share/applications -maxdepth 2 -name '*.desktop' 2>/dev/null | sort -u"
	out, err := exec.Command("/usr/bin/ssh", "-o", "BatchMode=yes", sshTarget, script).Output()
	if err != nil {
		return nil, fmt.Errorf("ssh list: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, "/") {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// FetchDesktopFiles batch-fetches multiple .desktop file contents via a single SSH call.
// Uses NUL as delimiter between entries to handle binary-safe parsing.
func FetchDesktopFiles(vmCfg VMConfig, paths []string) (map[string][]byte, error) {
	sshTarget := resolveSSHTarget("", vmCfg)
	if len(paths) == 0 {
		return nil, nil
	}

	// Build a shell script that cats all files with NUL delimiters.
	// Output: "---FILE---<NUL><path><NUL><content><NUL>"
	var script strings.Builder
	for _, p := range paths {
		script.WriteString("printf '%s\\x00%s\\x00' '---FILE---' '")
		script.WriteString(p)
		script.WriteString("'; cat '")
		script.WriteString(p)
		script.WriteString("'; printf '\\x00'")
		script.WriteString("; ")
	}

	out, err := exec.Command("/usr/bin/ssh", "-o", "BatchMode=yes", sshTarget, script.String()).Output()
	if err != nil {
		return nil, fmt.Errorf("ssh batch fetch: %w", err)
	}

	result := make(map[string][]byte)
	// Split by ---FILE---<NUL> delimiter
	delim := "---FILE---" + "\x00"
	parts := strings.Split(string(out), delim)
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		// Find next NUL: path<NUL>content<NUL>
		idx := strings.Index(part, "\x00")
		if idx < 0 {
			continue
		}
		path := part[:idx]
		content := part[idx+1:]
		// Remove trailing NUL from content
		if len(content) > 0 && content[len(content)-1] == '\x00' {
			content = content[:len(content)-1]
		}
		if path != "" && content != "" {
			result[path] = []byte(content)
		}
	}
	return result, nil
}

// FetchDesktopFile reads and returns the content of a .desktop file from the guest.
func FetchDesktopFile(vmName string, vmCfg VMConfig, path string) ([]byte, error) {
	sshTarget := resolveSSHTarget(vmName, vmCfg)
	out, err := exec.Command("/usr/bin/ssh", "-o", "BatchMode=yes", sshTarget, "cat", path).Output()
	if err != nil {
		return nil, fmt.Errorf("ssh cat %s: %w", path, err)
	}
	return out, nil
}

func ParseDesktopFile(content []byte) AppEntry {
	var name, exec, icon, comment, entryType string
	var keywords []string
	var noDisplay bool
	inAction := false

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Track section headers like [Desktop Action foo]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(line[1 : len(line)-1])
			if strings.HasPrefix(section, "desktop action ") {
				inAction = true
			} else {
				inAction = false
			}
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])

		switch key {
		case "name":
			// Only take Name from main [Desktop Entry] section, not Actions
			if !inAction {
				name = val
			}
		case "exec":
			if !inAction {
				exec = val
			}
		case "icon":
			if !inAction {
				icon = val
			}
		case "comment":
			if !inAction {
				comment = val
			}
		case "keywords":
			if !inAction {
				for _, k := range strings.Split(val, ";") {
					if k = strings.TrimSpace(k); k != "" {
						keywords = append(keywords, k)
					}
				}
			}
		case "nodisplay":
			if !inAction {
				noDisplay = strings.ToLower(val) == "true"
			}
		case "type":
			if !inAction {
				entryType = val
			}
		}
	}

	// Heuristic: Java AWT/Swing apps need X11 forwarding, not waypipe
	forwardMode := "waypipe"
	if strings.Contains(exec, "java ") || strings.Contains(exec, "/BurpSuite") {
		forwardMode = "x11"
	}

	return AppEntry{
		Name:        name,
		Exec:        exec,
		Icon:        icon,
		Comment:     comment,
		Keywords:    keywords,
		NoDisplay:   noDisplay,
		Type:        entryType,
		ForwardMode: forwardMode,
	}
}

// ListRunningVMs returns names of all running libvirt domains.
func ListRunningVMs() ([]string, error) {
	out, err := exec.Command("virsh", "-c", "qemu:///system", "list", "--name", "--state-running").Output()
	if err != nil {
		return nil, fmt.Errorf("virsh list: %w", err)
	}
	var names []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			names = append(names, name)
		}
	}
	return names, scanner.Err()
}

// resolveSSHTarget returns the SSH target string for a VM.
// Priority: vmCfg.SSHTarget > vmCfg.SSHUser@vmCfg.SSHHost:vmCfg.SSHPort > user@vmName
func resolveSSHTarget(vmName string, vmCfg VMConfig) string {
	if vmCfg.SSHTarget != "" {
		return vmCfg.SSHTarget
	}
	user := vmCfg.SSHUser
	if user == "" {
		user = "sec"
	}
	host := vmCfg.SSHHost
	if host == "" {
		host = vmName
	}
	port := ""
	if vmCfg.SSHPort != 0 && vmCfg.SSHPort != 22 {
		port = fmt.Sprintf(":%d", vmCfg.SSHPort)
	}
	return fmt.Sprintf("%s@%s%s", user, host, port)
}

// ParseDesktopName extracts the app name from a .desktop file path.
var desktopNameRe = regexp.MustCompile(`/([^/]+)\.desktop$`)

func ParseDesktopName(path string) string {
	m := desktopNameRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return path
	}
	return m[1]
}
