import QtQuick
import Quickshell
import Quickshell.Io

Item {
    id: root

    property var pluginService: null
    property string pluginId: "vm-launcher"

    property var cachedItems: []

    readonly property string _dmsVmBin: "/home/intox/.local/bin/dms-vm"

    FileView {
        id: configFile
        path: "/home/intox/.config/dms-waypipe/config.json"
        blockLoading: true
        preload: false
    }

    function loadItemsFromConfig() {
        var text = configFile.text();
        if (!text) return;
        try {
            var config = JSON.parse(text);
            var vms = config.vms || {};
            var items = [];

            for (var vmName in vms) {
                var vm = vms[vmName];
                var apps = vm.apps || [];
                for (var i = 0; i < apps.length; i++) {
                    var app = apps[i];
                    if (!app.Exec) continue;

                    var exec = app.Exec || "";

                    // Use x11: prefix for apps that need X11 forwarding (Java AWT/Swing)
                    var actionPrefix = (app.ForwardMode === "x11") ? "x11:" : "vm:";

                    items.push({
                        name: vmName + ": " + app.Name,
                        icon: app.Icon || "computer",
                        comment: app.Comment || "",
                        action: actionPrefix + vmName + ":" + exec,
                        categories: ["VM Apps"],
                        keywords: app.Keywords || []
                    });
                }
            }

            cachedItems = items;
            if (pluginService) pluginService.itemsChanged();
        } catch (e) {
            console.error("[vm-launcher] config parse error:", e);
        }
    }

    function getItems(query) {
        if (cachedItems.length === 0) {
            loadItemsFromConfig();
        }
        if (cachedItems.length === 0) {
            return [];
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
        var mode = parts[0]; // "vm" or "x11"
        var vmName = parts[1];
        var execCmd = parts.slice(2).join(":");
        if (mode === "x11") {
            Quickshell.execDetached(
                ["/bin/bash", "-c", _dmsVmBin + " launch --x11 " + vmName + " " + execCmd + " > /dev/null 2>&1"],
                function(out, err, code) {
                    if (code !== 0) console.error("[vm-launcher] x11 launch failed:", err);
                });
        } else {
            Quickshell.execDetached(
                ["/bin/bash", "-c", _dmsVmBin + " launch " + vmName + " " + execCmd + " > /dev/null 2>&1"],
                function(out, err, code) {
                    if (code !== 0) console.error("[vm-launcher] launch failed:", err);
                });
        }
    }
}
