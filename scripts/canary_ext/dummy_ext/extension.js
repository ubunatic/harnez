import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import St from 'gi://St';
import Clutter from 'gi://Clutter';

export default class CanaryDummyExtension extends Extension {
    enable() {
        console.log('[HARNEZ_CANARY_EXT] CanaryDummyExtension enabled successfully!');

        // Display a native GNOME notification banner inside the nested shell
        try {
            Main.notify('Harnez Canary Extension', 'Extension loaded successfully inside nested shell!');
        } catch (e) {
            console.error('[HARNEZ_CANARY_EXT] Main.notify failed:', e);
        }

        // Add a minimal button indicator to the top bar panel so it is visually verified
        this._indicator = new PanelMenu.Button(0.0, 'Harnez Canary Indicator', false);
        const icon = new St.Icon({
            icon_name: 'audio-input-microphone-symbolic',
            style_class: 'system-status-icon',
        });
        this._indicator.add_child(icon);

        Main.panel.addToStatusArea(this.uuid, this._indicator);
        console.log('[HARNEZ_CANARY_EXT] Status indicator added to Main.panel');
    }

    disable() {
        console.log('[HARNEZ_CANARY_EXT] CanaryDummyExtension disabled');
        if (this._indicator) {
            this._indicator.destroy();
            this._indicator = null;
        }
    }
}
