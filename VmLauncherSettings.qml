import QtQuick
import QtQuick.Layouts

import qs.Common
import qs.Modules.Plugins
import qs.Widgets

PluginSettings {
    id: root
    pluginId: "vm-launcher"

    // ------------------------------------------------------------
    // VM list editor
    // ------------------------------------------------------------

    property var _vms: ({})
    Component.onCompleted: {
        _loadFromGo();
    }

    function _loadFromGo() {
        // Load VM config from Go config file directly.
        Quickshell.execDetached(["/bin/bash", "-c",
            "if [ -f ~/.config/dms-waypipe/config.json ]; then " +
            "  cat ~/.config/dms-waypipe/config.json; " +
            "else echo '{}'; fi"],
            (out, err, code) => {
                if (code !== 0) { _vms = {}; return; }
                try {
                    const data = JSON.parse(out);
                    _vms = data.vms || {};
                    console.log("[vm-launcher] settings loaded", Object.keys(_vms).length, "VMs");
                } catch (e) { _vms = {}; }
            });
    }
    function _save() {
        pluginService.savePluginData(pluginId, "vms", JSON.stringify(_vms));
        // Sync to Go config file via a temp file to avoid shell quoting issues.
        const tmpFile = "/tmp/dms-vm-sync-" + Date.now() + ".json";
        const cleanVMs = {};
        for (const k of Object.keys(_vms)) {
            const v = Object.assign({}, _vms[k]);
            delete v.apps;
            cleanVMs[k] = v;
        }
        const payload = JSON.stringify({ vms: cleanVMs });
        Quickshell.execDetached(["bash", "-c",
            "cat > " + tmpFile + " << 'DMSVCFGEOF'\n" + payload + "\nDMSVCFGEOF\ndms-vm save-config < " + tmpFile + " && rm -f " + tmpFile],
            (out, err, code) => {
                if (code !== 0) console.warn("[vm-launcher] save-config failed:", err);
            });
    }

    // Regenerate Hyprland window_rules.lua after VM config changes.
    function _regenerateRules() {
        // Use $HOME via shell expansion; execDetached with a single string runs in a shell.
        const rulesPath = "$HOME/.config/hypr/plugins/dms_waypipe/window_rules.lua";
        Quickshell.execDetached('dms-vm generate-rules --write "' + rulesPath + '"', (out, err, code) => {
            if (code !== 0) {
                console.warn("[vm-launcher] generate-rules failed:", err);
            } else {
                console.log("[vm-launcher] window_rules.lua updated");
            }
        });
    }

    // ------------------------------------------------------------
    // Header
    // ------------------------------------------------------------

    Column {
        spacing: Theme.spacingM
        width: parent.width

        StyledText {
            text: "VM App Launcher"
            font.pixelSize: Theme.fontSizeLarge
            font.weight: Font.Bold
            color: Theme.surfaceText
        }

        StyledText {
            text: "Configure VMs to launch applications via waypipe. " +
                  "Spotlight queries matching a VM name will show that VM's apps."
            font.pixelSize: Theme.fontSizeSmall
            color: Theme.surfaceVariantText
            wrapMode: Text.WordWrap
        }

        HorizontalRule {}

        // ------------------------------------------------------------
        // Per-VM config
        // ------------------------------------------------------------

        StyledText {
            text: "Virtual Machines"
            font.pixelSize: Theme.fontSizeMedium
            font.weight: Font.Medium
            color: Theme.surfaceText
        }

        Repeater {
            model: Object.keys(_vms).length
            property var vmKeys: Object.keys(_vms)
            delegate: vmEditor {
                vmKey: vmKeys[index]
                vm: _vms[vmKeys[index]]
                onUpdated: (key, val) => {
                    _vms[key] = val;
                    _save();
                    _regenerateRules();
                }
                onDeleted: (key) => {
                    delete _vms[key];
                    _save();
                    _regenerateRules();
                }
            }
        }

        // Add VM button
        Button {
            text: "Add VM"
            icon: "add"
            onClicked: {
                const name = "new-vm-" + Date.now();
                _vms[name] = { name: name, color: "#ff7b54", apps: [], sshTarget: "" };
                _save();
                _regenerateRules();
            }
        }
    }

    // ------------------------------------------------------------
    // VM editor delegate
    // ------------------------------------------------------------

    component vmEditor: Column {
        property string vmKey
        property var vm
        signal updated(string key, var val)
        signal deleted(string key)

        readonly property int _rowH: 36

        Rectangle {
            width: parent.width
            height: childrenRect.height + Theme.spacingM * 2
            radius: Theme.cornerRadius
            color: Theme.surfaceContainerHigh

            Column {
                spacing: Theme.spacingS
                anchors {
                    fill: parent
                    margins: Theme.spacingM
                }

                // Name field
                RowLayout {
                    width: parent.width
                    spacing: Theme.spacingM

                    StyledText {
                        text: "Name:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 80
                    }

                    StringSetting {
                        settingKey: "name"
                        placeholder: "kali"
                        defaultValue: vm.name || ""
                        value: vm.name || ""
                        onValueChanged: {
                            vm.name = value;
                            updated(vmKey, vm);
                        }
                        Layout.fillWidth: true
                    }

                    // Color
                    StyledText {
                        text: "Color:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 50
                    }

                    StringSetting {
                        settingKey: "color"
                        placeholder: "#ff7b54"
                        defaultValue: vm.color || "#ff7b54"
                        value: vm.color || "#ff7b54"
                        onValueChanged: {
                            vm.color = value;
                            updated(vmKey, vm);
                        }
                        Layout.preferredWidth: 80
                    }
                }

                // SSH / vsock fields
                RowLayout {
                    width: parent.width
                    spacing: Theme.spacingM

                    StyledText {
                        text: "SSH Host:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 80
                    }

                    StringSetting {
                        settingKey: "sshHost"
                        placeholder: "127.0.0.1"
                        defaultValue: vm.sshHost || "127.0.0.1"
                        value: vm.sshHost || "127.0.0.1"
                        onValueChanged: {
                            vm.sshHost = value;
                            updated(vmKey, vm);
                        }
                        Layout.preferredWidth: 120
                    }

                    StyledText {
                        text: "SSH Port:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 70
                    }

                    StringSetting {
                        settingKey: "sshPort"
                        placeholder: "22"
                        defaultValue: (vm.sshPort || 22).toString()
                        value: (vm.sshPort || 22).toString()
                        onValueChanged: {
                            vm.sshPort = parseInt(value) || 22;
                            updated(vmKey, vm);
                        }
                        Layout.preferredWidth: 60
                    }

                    StyledText {
                        text: "User:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 50
                    }

                    StringSetting {
                        settingKey: "sshUser"
                        placeholder: "root"
                        defaultValue: vm.sshUser || "root"
                        value: vm.sshUser || "root"
                        onValueChanged: {
                            vm.sshUser = value;
                            updated(vmKey, vm);
                        }
                        Layout.preferredWidth: 80
                    }
                }

                // sshTarget field (overrides individual SSH fields when set)
                RowLayout {
                    width: parent.width
                    spacing: Theme.spacingM

                    StyledText {
                        text: "SSH Target:"
                        font.pixelSize: Theme.fontSizeSmall
                        color: Theme.surfaceVariantText
                        Layout.preferredWidth: 80
                    }

                    StringSetting {
                        settingKey: "sshTarget"
                        placeholder: "e.g. sec@vsock%3 (overrides host/port/user)"
                        defaultValue: vm.sshTarget || ""
                        value: vm.sshTarget || ""
                        onValueChanged: {
                            vm.sshTarget = value;
                            updated(vmKey, vm);
                        }
                        Layout.fillWidth: true
                    }
                }

                // Actions
                Row {
                    spacing: Theme.spacingS

                    Button {
                        text: "Refresh Apps"
                        icon: "refresh"
                        onClicked: {
                            Quickshell.execDetached(["dms-vm", "refresh-apps", vmKey], (out, err, code) => {
                                if (code !== 0) {
                                    ToastService.showError("Refresh", "Failed: " + err);
                                } else {
                                    ToastService.showInfo("Refresh", "Apps refreshed for " + vm.name);
                                }
                            });
                        }
                    }

                    Button {
                        text: "Remove"
                        icon: "delete"
                        onClicked: deleted(vmKey)
                    }
                }
            }
        }
    }
}
