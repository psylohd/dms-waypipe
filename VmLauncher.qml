import QtQuick

import Quickshell

Item {
    id: root

    property var pluginService: null
    property string pluginId: "vm-launcher"

    property var cachedItems: [
        { name: "sec: chkrootkit", icon: "chkrootkit", comment: "Locally check for signs of a rootkit", action: "vm:sec:bash -c \"pkexec chkrootkit ; read -p 'Press ENTER to exit'\"", categories: ["VM Apps"], keywords: [] },
        { name: "sec: VSCodium", icon: "vscodium", comment: "Code Editing. Redefined.", action: "vm:sec:/usr/share/codium/codium --ozone-platform=wayland --disable-gpu", categories: ["VM Apps"], keywords: ["vscodium","codium","vscode"] },
        { name: "sec: Ghostty", icon: "com.mitchellh.ghostty", comment: "A terminal emulator", action: "vm:sec:/usr/bin/ghostty --gtk-single-instance=true", categories: ["VM Apps"], keywords: ["terminal","tty","pty"] },
        { name: "sec: EtherApe", icon: "etherape", comment: "Graphical Network Monitor", action: "vm:sec:etherape", categories: ["VM Apps"], keywords: ["network","ethernet","sniff"] },
        { name: "sec: Ettercap", icon: "ettercap", comment: "Ettercap is a multipurpose sniffer/content filter", action: "vm:sec:ettercap -G", categories: ["VM Apps"], keywords: ["network","sniffer","security"] },
        { name: "sec: GParted", icon: "gparted", comment: "GNOME partition editor", action: "vm:sec:/usr/bin/gparted", categories: ["VM Apps"], keywords: ["partition","disk","editor"] },
        { name: "sec: Htop", icon: "htop", comment: "Interactive process viewer", action: "vm:sec:htop", categories: ["VM Apps"], keywords: ["process","system","monitor"] },
        { name: "sec: Kitty", icon: "kitty", comment: "A fast, GPU based terminal emulator", action: "vm:sec:kitty", categories: ["VM Apps"], keywords: ["terminal","emulator","gpu"] },
        { name: "sec: LibreOffice Base", icon: "libreoffice-base", comment: "Database", action: "vm:sec:libreoffice --base", categories: ["VM Apps"], keywords: ["database","office"] },
        { name: "sec: LibreOffice Calc", icon: "libreoffice-calc", comment: "Spreadsheet", action: "vm:sec:libreoffice --calc", categories: ["VM Apps"], keywords: ["spreadsheet","office"] },
        { name: "sec: LibreOffice Writer", icon: "libreoffice-writer", comment: "Word processor", action: "vm:sec:libreoffice --writer", categories: ["VM Apps"], keywords: ["word","processor","office"] },
        { name: "sec: Neovim", icon: "nvim", comment: "Neovim", action: "vm:sec:neovim", categories: ["VM Apps"], keywords: ["editor","text","vim"] },
        { name: "sec: Files", icon: "org.gnome.Nautilus", comment: "Access and organize files", action: "vm:sec:nautilus --new-window", categories: ["VM Apps"], keywords: ["files","nautilus","folder","manager","explore","filesystem"] },
        { name: "sec: Zen Browser", icon: "app.zen_browser.zen", comment: "A fast, private and secure web browser", action: "vm:sec:/usr/bin/flatpak run app.zen_browser.zen", categories: ["VM Apps"], keywords: ["browser","web","flatpak"] }
    ]

    readonly property string _dmsVmBin: "/home/intox/.local/bin/dms-vm"

    function getItems(query) {
        if (cachedItems.length === 0) {
            return [{ name: "VM Apps (loading...)", icon: "computer", comment: "Loading...",
                      action: "", categories: ["VM Apps"], keywords: [] }];
        }

        var search = query;
        var colon = query.indexOf(":");
        if (colon >= 0) search = query.slice(colon + 1);
        var lower = search.toLowerCase();

        var filtered = cachedItems.filter(function(item) {
            if (!search) return true;
            return (item.name && item.name.toLowerCase().indexOf(lower) >= 0) ||
                   (item.comment && item.comment.toLowerCase().indexOf(lower) >= 0) ||
                   (item.keywords && item.keywords.some(function(k) {
                       return k.toLowerCase().indexOf(lower) >= 0;
                   }));
        });

        return filtered;
    }

    function executeItem(item) {
        if (!item || !item.action) return;
        var parts = item.action.split(":");
        if (parts.length < 3) return;
        var vmName = parts[1];
        var execCmd = parts.slice(2).join(":");
        Quickshell.execDetached(
            ["/bin/bash", "-c", _dmsVmBin + " launch " + vmName + " " + execCmd + " > /dev/null 2>&1"],
            function(out, err, code) {
                if (code !== 0) console.error("[vm-launcher] launch failed:", err);
            });
    }
}
