package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/intox/dms-vm-launcher/vmlib"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list-running":
		vms, err := vmlib.ListRunningVMs()
		if err != nil {
			fmt.Fprintln(os.Stderr, "vm: list-running:", err)
			os.Exit(1)
		}
		for _, name := range vms {
			fmt.Println(name)
		}

	case "list-apps":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dms-vm list-apps <vmname>")
			os.Exit(1)
		}
		runListApps(os.Args[2])

	case "launch":
		useX11 := false
		argStart := 2
		if os.Args[2] == "--x11" {
			useX11 = true
			argStart = 3
		}
		if len(os.Args) < argStart+2 {
			fmt.Fprintln(os.Stderr, "usage: dms-vm launch [--x11] <vmname> <exec>")
			os.Exit(1)
		}
		vmName := os.Args[argStart]
		execCmd := strings.Join(os.Args[argStart+1:], " ")
		runLaunch(vmName, execCmd, useX11)

	case "list-vms":
		runListVMs()

	case "read-result":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: dms-vm read-result <file>")
			os.Exit(1)
		}
		runReadResult(os.Args[2])

	case "refresh":
		cfgPath := configPath()
		vmName := ""
		if len(os.Args) >= 3 {
			vmName = os.Args[2]
		}
		runRefreshApps(cfgPath, vmName)

	case "generate-rules":
		cfgPath := configPath()
		writePath := ""
		args := os.Args[2:]
		for i, a := range args {
			if a == "--write" && i+1 < len(args) {
				writePath = args[i+1]
				args = append(args[:i], args[i+2:]...)
				break
			}
		}
		if len(args) >= 1 {
			cfgPath = args[0]
		}
		runGenerateRules(cfgPath, writePath)

	case "save-config":
		runSaveConfig()

	case "spotlist":
		runSpotlist()

	default:
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `dms-vm - VM App Launcher for DankMaterialShell

  dms-vm list-running        List running VMs (from virsh)
  dms-vm list-apps <vmname>    List installed applications in a VM
  dms-vm launch <vmname> <exec> Launch an application inside a VM
`)
}

func configPath() string {
	return vmlib.DefaultConfigPath()
}

func loadVMConfig() *vmlib.Config {
	cfg, err := vmlib.LoadConfig(configPath())
	if err != nil {
		log.Printf("[vm-launcher] warning: could not load config: %v", err)
		return &vmlib.Config{VMs: map[string]vmlib.VMConfig{}}
	}
	return cfg
}

func runListVMs() {
	cfg := loadVMConfig()
	type vmInfo struct {
		Name      string `json:"name"`
		Color     string `json:"color"`
		SSHTarget string `json:"sshTarget"`
	}
	var out []vmInfo
	for _, vm := range cfg.VMs {
		out = append(out, vmInfo{
			Name:      vm.Name,
			Color:     vm.Color,
			SSHTarget: vm.SSHTarget,
		})
	}

	cachePath := configPath() + ".cache"
	m := map[string]interface{}{"vms": out}
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		os.WriteFile(cachePath, data, 0644)
	}

	json.NewEncoder(os.Stdout).Encode(out)
}

func runReadResult(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-result:", err)
		os.Exit(1)
	}
	// Remove trailing "EXIT:<code>" line
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var result []string
	for _, line := range lines {
		if strings.HasPrefix(line, "EXIT:") {
			break
		}
		result = append(result, line)
	}
	os.Stdout.Write([]byte(strings.Join(result, "\n") + "\n"))
}

func runListApps(vmName string) {
	cfg := loadVMConfig()
	vmCfg, ok := cfg.VMs[vmName]
	if !ok {
		fmt.Fprintln(os.Stderr, "vm: unknown VM:", vmName)
		os.Exit(1)
	}

	paths, err := vmlib.ListGuestApps(vmName, vmCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vm: ListGuestApps:", err)
		os.Exit(1)
	}

	contents, err := vmlib.FetchDesktopFiles(vmCfg, paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vm: FetchDesktopFiles:", err)
		os.Exit(1)
	}

	var apps []vmlib.AppEntry
	seen := make(map[string]bool)
	for _, path := range paths {
		name := vmlib.ParseDesktopName(path)
		if seen[name] {
			continue
		}
		seen[name] = true

		content, ok := contents[path]
		if !ok {
			continue
		}

		entry := vmlib.ParseDesktopFile(content)
		if entry.Name == "" {
			entry.Name = name
		}
		if entry.Exec == "" {
			continue
		}
		apps = append(apps, entry)
	}

	json.NewEncoder(os.Stdout).Encode(apps)
}

func runLaunch(vmName, execCmd string, useX11 bool) {
	cfg := loadVMConfig()
	vmCfg, ok := cfg.VMs[vmName]
	if !ok {
		fmt.Fprintln(os.Stderr, "vm: unknown VM:", vmName)
		os.Exit(1)
	}

	var result *vmlib.LaunchResult
	var err error
	if useX11 {
		result, err = vmlib.LaunchAppX11(vmName, execCmd, vmCfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vm: launch (x11):", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "x11 pid=%d title=%q\n", result.PID, result.WindowTitle)
	} else {
		result, err = vmlib.LaunchApp(vmName, execCmd, vmCfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "vm: launch:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "waypipe pid=%d title=%q\n", result.PID, result.WindowTitle)
	}
}
func runSpotlist() {
	running, err := vmlib.ListRunningVMs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vm: list-running:", err)
		os.Exit(1)
	}
	if len(running) == 0 {
		json.NewEncoder(os.Stdout).Encode([]interface{}{})
		return
	}

	cfg := loadVMConfig()

	type SpotItem struct {
		Name       string   `json:"name"`
		Icon       string   `json:"icon"`
		Comment    string   `json:"comment,omitempty"`
		Action     string   `json:"action"`
		Categories []string `json:"categories"`
		Keywords   []string `json:"keywords,omitempty"`
	}

	// iconRemap maps guest icon names to Papirus icon names where they differ.
	iconRemap := map[string]string{
		"vscodium": "vscodium",
		"codium":   "vscodium",
		"firefox":  "firefox",
		"ghostty":  "com.mitchellh.ghostty",
	}

	shouldShow := func(entry vmlib.AppEntry, path string) bool {
		// Skip NoDisplay entries (URL handlers, background services, etc.)
		if entry.NoDisplay {
			return false
		}
		// Skip non-Application types
		if entry.Type != "" && entry.Type != "Application" {
			return false
		}
		// Skip URL handlers (MimeType=x-scheme-handler or exec contains %U with no other args)
		exec := entry.Exec
		if strings.Contains(exec, "%U") && !strings.Contains(exec, "%F") && !strings.Contains(exec, "%f") {
			// Likely a URL handler, skip unless it's the main app entry
			// Check if this path looks like a main app (not a -url-handler variant)
			if strings.Contains(path, "-url-handler") || strings.Contains(path, "url-handler") {
				return false
			}
			if strings.Contains(exec, "--open-url") || strings.Contains(exec, "-url-handler") {
				return false
			}
		}
		// Skip entries that are clearly auxiliary tools
		loweredName := strings.ToLower(entry.Name)
		if strings.Contains(loweredName, "extension") ||
			strings.Contains(loweredName, "plugin") ||
			strings.Contains(loweredName, "module") ||
			strings.Contains(loweredName, "library") ||
			strings.Contains(loweredName, "daemon") ||
			strings.Contains(loweredName, "background") {
			return false
		}
		return true
	}

	mapIcon := func(icon string) string {
		if icon == "" {
			return "computer"
		}
		// If icon exists as-is, use it
		// Otherwise check remap table
		if remapped, ok := iconRemap[icon]; ok {
			return remapped
		}
		return icon
	}

	allItems := []SpotItem{}

	for _, vmName := range running {
		vmCfg, ok := cfg.VMs[vmName]
		if !ok {
			fmt.Fprintf(os.Stderr, "vm: spotlist: no config for running VM %q, skipping\n", vmName)
			continue
		}

		paths, err := vmlib.ListGuestApps(vmName, vmCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vm: spotlist: ListGuestApps %s: %v\n", vmName, err)
			continue
		}

		contents, err := vmlib.FetchDesktopFiles(vmCfg, paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vm: spotlist: FetchDesktopFiles %s: %v\n", vmName, err)
			continue
		}

		seen := make(map[string]bool)
		for _, path := range paths {
			name := vmlib.ParseDesktopName(path)
			if seen[name] {
				continue
			}

			content, ok := contents[path]
			if !ok {
				continue
			}

			entry := vmlib.ParseDesktopFile(content)
			if entry.Name == "" {
				entry.Name = name
			}
			if entry.Exec == "" {
				continue
			}
			if !shouldShow(entry, path) {
				continue
			}

			seen[name] = true

			keywords := entry.Keywords
			if keywords == nil {
				keywords = []string{}
			}

		allItems = append(allItems, SpotItem{
			Name:       vmName + ": " + entry.Name,
			Icon:       mapIcon(entry.Icon),
			Comment:    entry.Comment,
			Action:     "vm:" + vmName + ":" + entry.Exec,
			Categories: []string{"VM Apps"},
			Keywords:   keywords,
		})
	}
}

	json.NewEncoder(os.Stdout).Encode(allItems)
}

func runRefreshApps(cfgPath, filterVM string) {
	cfg, err := vmlib.LoadConfig(cfgPath)
	if err != nil {
		log.Printf("load config: %v", err)
		return
	}

	iconRemap := map[string]string{
		"vscodium": "vscodium",
		"codium":   "vscodium",
		"firefox":  "firefox",
		"ghostty":  "com.mitchellh.ghostty",
	}

	shouldShow := func(entry vmlib.AppEntry, path string) bool {
		if entry.NoDisplay {
			return false
		}
		if strings.Contains(entry.Exec, "%U") && !strings.Contains(entry.Exec, "%F") && !strings.Contains(entry.Exec, "%f") {
			if strings.Contains(path, "-url-handler") || strings.Contains(path, "url-handler") {
				return false
			}
			if strings.Contains(entry.Exec, "--open-url") || strings.Contains(entry.Exec, "-url-handler") {
				return false
			}
		}
		loweredName := strings.ToLower(entry.Name)
		if strings.Contains(loweredName, "extension") ||
			strings.Contains(loweredName, "plugin") ||
			strings.Contains(loweredName, "module") ||
			strings.Contains(loweredName, "library") ||
			strings.Contains(loweredName, "daemon") ||
			strings.Contains(loweredName, "background") {
			return false
		}
		return true
	}

	mapIcon := func(icon string) string {
		if icon == "" {
			return "computer"
		}
		if remapped, ok := iconRemap[icon]; ok {
			return remapped
		}
		return icon
	}

	var vmsToRefresh []string
	if filterVM != "" {
		vmsToRefresh = []string{filterVM}
	} else {
		vmsToRefresh, err = vmlib.ListRunningVMs()
		if err != nil {
			log.Printf("list-running: %v", err)
			return
		}
	}

	dirty := false
	for _, vmName := range vmsToRefresh {
		vmCfg, ok := cfg.VMs[vmName]
		if !ok {
			fmt.Fprintf(os.Stderr, "vm: refresh: no config for VM %q\n", vmName)
			continue
		}

		paths, err := vmlib.ListGuestApps(vmName, vmCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vm: refresh: ListGuestApps %s: %v\n", vmName, err)
			continue
		}

		contents, err := vmlib.FetchDesktopFiles(vmCfg, paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vm: refresh: FetchDesktopFiles %s: %v\n", vmName, err)
			continue
		}

		seen := make(map[string]bool)
		var apps []vmlib.AppEntry
		for _, path := range paths {
			name := vmlib.ParseDesktopName(path)
			if seen[name] {
				continue
			}
			content, ok := contents[path]
			if !ok {
				continue
			}
			entry := vmlib.ParseDesktopFile(content)
			if entry.Name == "" {
				entry.Name = name
			}
			if entry.Exec == "" {
				continue
			}
			if !shouldShow(entry, path) {
				continue
			}
			seen[name] = true
			entry.Icon = mapIcon(entry.Icon)
			apps = append(apps, entry)
		}

		vmCfg.Apps = apps
		cfg.VMs[vmName] = vmCfg
		dirty = true
		fmt.Printf("vm: refresh: %s: %d apps\n", vmName, len(apps))
	}

	if dirty {
		if err := vmlib.SaveConfig(cfgPath, cfg); err != nil {
			log.Printf("save config: %v", err)
			os.Exit(1)
		}
		fmt.Println("vm: refresh: saved to", cfgPath)
	}
}

func runSaveConfig() {
	cfg := &vmlib.Config{}
	if err := json.NewDecoder(os.Stdin).Decode(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "save-config: parse stdin:", err)
		os.Exit(1)
	}
	if err := vmlib.SaveConfig(configPath(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "save-config: write:", err)
		os.Exit(1)
	}
}

func runGenerateRules(cfgPath, writePath string) {
	cfg, err := vmlib.LoadConfig(cfgPath)
	if err != nil {
		log.Printf("load config: %v", err)
	}

	var out io.Writer = os.Stdout
	if writePath != "" {
		f, err := os.Create(writePath)
		if err != nil {
			log.Fatalf("create %s: %v", writePath, err)
		}
		defer f.Close()
		out = f
	}

	fmt.Fprintln(out, "-- window_rules.lua: auto-generated by dms-vm generate-rules")
	fmt.Fprintln(out, "")

	if cfg != nil {
		for name, vm := range cfg.VMs {
			color := vm.Color
			if color == "" {
				color = "#ff7b54"
			}
			// Support "focused_color unfocused_color" format
			colors := strings.Fields(color)
			focusedRGB := strings.TrimPrefix(colors[0], "#")
			unfocusedRGB := focusedRGB
			if len(colors) >= 2 {
				unfocusedRGB = strings.TrimPrefix(colors[1], "#")
			}
			pattern := fmt.Sprintf("^(.*VM:%s.*)$", name)
			fmt.Fprintf(out, "hl.window_rule({\n")
			fmt.Fprintf(out, "    name = \"vm-%s\",\n", name)
			fmt.Fprintf(out, "    match = { initial_title = %q },\n", pattern)
			fmt.Fprintf(out, "    border_color = \"rgb(%s) rgb(%s)\",\n", focusedRGB, unfocusedRGB)
			fmt.Fprintf(out, "})\n\n")
		}
	}
	fmt.Fprintln(out, "return {}")
}
