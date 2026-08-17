import { Extension, gettext as _ } from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import * as PanelMenu from 'resource:///org/gnome/shell/ui/panelMenu.js';
import * as PopupMenu from 'resource:///org/gnome/shell/ui/popupMenu.js';
import * as Slider from 'resource:///org/gnome/shell/ui/slider.js';
import St from 'gi://St';
import Clutter from 'gi://Clutter';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import Meta from 'gi://Meta';

async function runCommand(programName, args, cancellable = null) {
    let program = GLib.find_program_in_path(programName);
    if (!program) {
        const home = GLib.get_home_dir();
        const candidates = [
            GLib.build_filenamev([home, '.local', 'bin', programName]),
            GLib.build_filenamev([home, 'go', 'bin', programName]),
            GLib.build_filenamev([home, 'projects', 'harnez', programName]),
            `/usr/local/bin/${programName}`,
            `/usr/bin/${programName}`,
        ];
        for (const cand of candidates) {
            if (GLib.file_test(cand, GLib.FileTest.IS_EXECUTABLE)) {
                program = cand;
                break;
            }
        }
        if (!program) {
            program = programName;
        }
    }

    try {
        const launcher = new Gio.SubprocessLauncher({
            flags: Gio.SubprocessFlags.STDOUT_PIPE | Gio.SubprocessFlags.STDERR_PIPE,
        });
        const proc = launcher.spawnv([program, ...args]);
        const [stdoutBytes, stderrBytes] = await new Promise((resolve, reject) => {
            proc.communicate_utf8_async(null, cancellable, (obj, res) => {
                try {
                    const result = obj.communicate_utf8_finish(res);
                    resolve(result);
                } catch (e) {
                    reject(e);
                }
            });
        });
        const res = {
            success: proc.get_successful(),
            exitCode: proc.get_exit_status(),
            stdout: stdoutBytes ? stdoutBytes.trim() : '',
            stderr: stderrBytes ? stderrBytes.trim() : '',
        };
        console.log(`[HARNEZ_VOICE_INPUT_EXT] Subprocess ran: ${program} ${args.join(' ')} -> success=${res.success} stdout="${res.stdout}" stderr="${res.stderr}"`);
        return res;
    } catch (err) {
        console.error(`[HARNEZ_VOICE_INPUT_EXT] Subprocess error (${program} ${args.join(' ')}):`, err);
        return {
            success: false,
            exitCode: -1,
            stdout: '',
            stderr: err.message || String(err),
        };
    }
}

async function runHarnez(args, cancellable = null) {
    return runCommand('harnez', args, cancellable);
}

export default class VoiceInputExtension extends Extension {
    enable() {
        console.log('[HARNEZ_VOICE_INPUT_EXT] VoiceInputExtension enabled successfully!');

        this._capturedWindow = null;
        this._typeDelayMs = 0;
        this._currentMode = 'unknown';

        // 1. Create Top Bar Indicator Button
        this._indicator = new PanelMenu.Button(0.0, this.metadata.name, false);
        this._icon = new St.Icon({
            icon_name: 'audio-input-microphone-symbolic',
            style_class: 'system-status-icon',
        });
        this._indicator.add_child(this._icon);

        // Intercept click on the top-bar indicator:
        // - If recording: Left-click immediately stops recording (no menu open). Right-click opens menu.
        // - If idle: Left/right click opens menu normally.
        this._indicator.connect('button-press-event', (actor, event) => {
            const button = event.get_button();
            if (this._isRecording && button === 1) {
                // Primary left click during active recording -> Instant Stop
                this._stopRecordingDirectly();
                return Clutter.EVENT_STOP;
            }
            // Allow menu to toggle for right click or when idle
            return Clutter.EVENT_PROPAGATE;
        });

        // 2. Build Menu Layout
        this._buildMenu();

        // 3. Connect Menu open-state for focus capture and data refresh
        this._openStateSignal = this._indicator.menu.connect('open-state-changed', (menu, isOpen) => {
            if (isOpen) {
                this._onMenuOpened();
            } else {
                this._onMenuClosed();
            }
        });

        // Add indicator to the top bar (right status area)
        Main.panel.addToStatusArea(this.uuid, this._indicator, 1, 'right');

        // 4. Start background state poller (polls every 500ms) to track global hotkey (Ctrl+Super+X)
        this._pollTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 500, () => {
            this._checkBackgroundState();
            return GLib.SOURCE_CONTINUE;
        });

        try {
            Main.notify('Harnez Voice Input', 'Voice Input companion extension activated');
        } catch (e) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Notification error:', e);
        }
    }

    disable() {
        console.log('[HARNEZ_VOICE_INPUT_EXT] VoiceInputExtension disabled');
        if (this._pollTimerId) {
            GLib.source_remove(this._pollTimerId);
            this._pollTimerId = 0;
        }
        if (this._openStateSignal && this._indicator) {
            this._indicator.menu.disconnect(this._openStateSignal);
            this._openStateSignal = 0;
        }
        if (this._indicator) {
            this._indicator.destroy();
            this._indicator = null;
        }
        this._capturedWindow = null;
    }

    _checkBackgroundState() {
        const runtimeDir = GLib.get_user_runtime_dir();
        const stateFile = GLib.build_filenamev([runtimeDir, 'voxtype', 'state']);
        let isRecording = false;

        if (GLib.file_test(stateFile, GLib.FileTest.EXISTS)) {
            try {
                const [ok, contents] = GLib.file_get_contents(stateFile);
                if (ok && contents) {
                    const text = new TextDecoder().decode(contents).trim().toLowerCase();
                    isRecording = text.includes('recording') || text.includes('listening');
                }
            } catch (e) {
                console.error('[HARNEZ_VOICE_INPUT_EXT] Error reading state file:', e);
            }
        }

        if (this._isRecording !== isRecording) {
            console.log(`[HARNEZ_VOICE_INPUT_EXT] State transition detected: isRecording changed from ${this._isRecording} to ${isRecording} (stateFile: ${stateFile})`);
            this._isRecording = isRecording;
            this._updateRecordingUI(isRecording);
        }
    }

    _updateRecordingUI(isRecording) {
        if (isRecording) {
            if (this._recordToggleBtn) {
                this._recordToggleBtn.label = '⏹ Stop Recording';
                this._recordToggleBtn.add_style_class_name('recording');
            }
            if (this._icon) {
                this._icon.icon_name = 'media-record-symbolic';
                this._icon.add_style_class_name('voice-input-icon-recording');
            }
        } else {
            if (this._recordToggleBtn) {
                this._recordToggleBtn.label = '🎙 Start Recording';
                this._recordToggleBtn.remove_style_class_name('recording');
            }
            if (this._icon) {
                this._icon.icon_name = 'audio-input-microphone-symbolic';
                this._icon.remove_style_class_name('voice-input-icon-recording');
            }
        }
    }

    _onMenuOpened() {
        // Step 1: Capture focused window before menu interactions occur
        this._capturedWindow = global.display.get_focus_window();
        const winTitle = this._capturedWindow ? this._capturedWindow.get_title() : 'none';
        console.log(`[HARNEZ_VOICE_INPUT_EXT] Menu opened; captured target window: "${winTitle}"`);

        // Step 2: Refresh async status
        this._refreshAllState();
    }

    _onMenuClosed() {
        // Keep capturedWindow until next open or retype execution
    }

    _buildMenu() {
        const menu = this._indicator.menu;

        // ── Section 0: Live Recording Toggle ──
        const recordSection = new PopupMenu.PopupBaseMenuItem({
            reactive: false,
            can_focus: false,
            style_class: 'voice-input-section',
        });
        const recordLayout = new St.BoxLayout({ vertical: true, x_expand: true });
        
        this._recordToggleBtn = new St.Button({
            label: '🎙 Start Recording',
            style_class: 'voice-input-record-btn',
            x_expand: true,
            can_focus: true,
        });
        this._recordToggleBtn.connect('clicked', () => this._toggleRecording());
        recordLayout.add_child(this._recordToggleBtn);
        recordSection.add_child(recordLayout);
        menu.addMenuItem(recordSection);

        menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // ── Section 1: Mode Selection ──
        const modeSection = new PopupMenu.PopupBaseMenuItem({
            reactive: false,
            can_focus: false,
            style_class: 'voice-input-section',
        });
        const modeLayout = new St.BoxLayout({ vertical: true, x_expand: true });
        
        const modeHeaderBox = new St.BoxLayout({ x_expand: true });
        this._modeTitleLabel = new St.Label({
            text: 'Voice Dictation Mode',
            style_class: 'voice-input-section-title',
            y_align: Clutter.ActorAlign.CENTER,
        });
        this._modeStatusLabel = new St.Label({
            text: '(checking...)',
            style_class: 'voice-input-note',
            x_align: Clutter.ActorAlign.END,
            x_expand: true,
            y_align: Clutter.ActorAlign.CENTER,
        });
        modeHeaderBox.add_child(this._modeTitleLabel);
        modeHeaderBox.add_child(this._modeStatusLabel);

        const modeBtnBox = new St.BoxLayout({
            style_class: 'voice-input-mode-box',
            x_expand: true,
        });

        this._batchModeBtn = new St.Button({
            label: 'Batch (Whisper)',
            style_class: 'voice-input-mode-button',
            x_expand: true,
            can_focus: true,
        });
        this._batchModeBtn.connect('clicked', () => this._switchMode('batch'));

        this._streamingModeBtn = new St.Button({
            label: 'Streaming (Parakeet)',
            style_class: 'voice-input-mode-button',
            x_expand: true,
            can_focus: true,
        });
        this._streamingModeBtn.connect('clicked', () => this._switchMode('streaming'));

        modeBtnBox.add_child(this._batchModeBtn);
        modeBtnBox.add_child(this._streamingModeBtn);

        modeLayout.add_child(modeHeaderBox);
        modeLayout.add_child(modeBtnBox);
        modeSection.add_child(modeLayout);
        menu.addMenuItem(modeSection);

        menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // ── Section 2: Typing Speed / Delay Config ──
        const delaySection = new PopupMenu.PopupBaseMenuItem({
            reactive: false,
            can_focus: false,
            style_class: 'voice-input-section',
        });
        const delayLayout = new St.BoxLayout({ vertical: true, x_expand: true });

        const delayHeaderBox = new St.BoxLayout({ x_expand: true });
        this._delayTitleLabel = new St.Label({
            text: 'Typing Delay',
            style_class: 'voice-input-section-title',
        });
        this._delayValueLabel = new St.Label({
            text: '0 ms',
            style_class: 'voice-input-section-title',
            x_align: Clutter.ActorAlign.END,
            x_expand: true,
        });
        delayHeaderBox.add_child(this._delayTitleLabel);
        delayHeaderBox.add_child(this._delayValueLabel);

        this._delaySlider = new Slider.Slider(0);
        this._delaySlider.connect('notify::value', () => this._onSliderMoved());
        this._delaySlider.connect('drag-end', () => this._onSliderCommitted());

        const presetBox = new St.BoxLayout({
            style_class: 'voice-input-slider-box',
            x_expand: true,
        });
        const presets = [0, 15, 30, 60, 100];
        presets.forEach(ms => {
            const btn = new St.Button({
                label: `${ms}ms`,
                style_class: 'voice-input-action-btn',
                x_expand: true,
            });
            btn.connect('clicked', () => this._setDelayMs(ms));
            presetBox.add_child(btn);
        });

        const delayNote = new St.Label({
            text: 'Restart voxtype daemon to apply delay changes',
            style_class: 'voice-input-note',
        });

        delayLayout.add_child(delayHeaderBox);
        delayLayout.add_child(this._delaySlider);
        delayLayout.add_child(presetBox);
        delayLayout.add_child(delayNote);
        delaySection.add_child(delayLayout);
        menu.addMenuItem(delaySection);

        menu.addMenuItem(new PopupMenu.PopupSeparatorMenuItem());

        // ── Section 3: Recent Transcripts Header & Clear ──
        const historyHeaderItem = new PopupMenu.PopupBaseMenuItem({
            reactive: false,
            can_focus: false,
            style_class: 'voice-input-section',
        });
        const historyHeaderLayout = new St.BoxLayout({ x_expand: true });

        const historyTitle = new St.Label({
            text: 'Recent Transcripts',
            style_class: 'voice-input-section-title',
            y_align: Clutter.ActorAlign.CENTER,
            x_expand: true,
        });

        this._clearBtn = new St.Button({
            label: 'Clear',
            style_class: 'voice-input-clear-btn',
            y_align: Clutter.ActorAlign.CENTER,
        });
        this._clearBtn.connect('clicked', () => this._onClearHistory());

        historyHeaderLayout.add_child(historyTitle);
        historyHeaderLayout.add_child(this._clearBtn);
        historyHeaderItem.add_child(historyHeaderLayout);
        menu.addMenuItem(historyHeaderItem);

        // History Scroll Container
        this._historySection = new PopupMenu.PopupBaseMenuItem({
            reactive: false,
            can_focus: false,
            style_class: 'voice-input-section',
        });
        this._historyScrollView = new St.ScrollView({
            style_class: 'voice-input-history-scroll',
            x_expand: true,
        });
        this._historyScrollView.set_policy(St.PolicyType.NEVER, St.PolicyType.AUTOMATIC);

        this._historyListBox = new St.BoxLayout({
            vertical: true,
            x_expand: true,
            style_class: 'voice-input-history-box',
        });
        this._historyScrollView.set_child(this._historyListBox);
        this._historySection.add_child(this._historyScrollView);
        menu.addMenuItem(this._historySection);
    }

    async _refreshAllState() {
        await Promise.all([
            this._refreshRecordingStatus(),
            this._refreshMode(),
            this._refreshConfig(),
            this._refreshHistory(),
        ]);
    }

    async _refreshRecordingStatus() {
        const res = await runHarnez(['tools', 'voice-input', 'record', 'status']);
        let isRecording = false;
        if (res.success && res.stdout) {
            const out = res.stdout.toLowerCase().trim();
            isRecording = out === 'recording';
        }
        this._isRecording = isRecording;
        this._updateRecordingUI(isRecording);
    }

    async _stopRecordingDirectly() {
        console.log('[HARNEZ_VOICE_INPUT_EXT] Direct 1-click stop requested from panel icon');
        // Optimistic UI update immediately
        this._isRecording = false;
        this._updateRecordingUI(false);
        if (this._indicator && this._indicator.menu.isOpen) {
            this._indicator.menu.close();
        }

        const res = await runHarnez(['tools', 'voice-input', 'record', 'stop']);
        if (!res.success) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Direct stop failed:', res.stderr);
        }
        await this._refreshRecordingStatus();
    }

    async _toggleRecording() {
        const wasRecording = this._isRecording;
        const targetWin = this._capturedWindow;
        
        // Optimistic UI update immediately
        this._isRecording = !wasRecording;
        this._updateRecordingUI(this._isRecording);

        // Immediately close menu so focus returns to the target window
        this._indicator.menu.close();
        if (!wasRecording && targetWin) {
            try {
                targetWin.activate(global.get_current_time());
            } catch (e) {
                console.error('[HARNEZ_VOICE_INPUT_EXT] Focus activation error on record start:', e);
            }
        }

        const action = wasRecording ? 'stop' : 'start';
        const res = await runHarnez(['tools', 'voice-input', 'record', action]);
        if (!res.success) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Record action failed:', res.stderr);
        }
        await this._refreshRecordingStatus();
    }

    async _refreshMode() {
        const res = await runHarnez(['tools', 'voice-input', 'mode']);
        let mode = 'neither';
        if (res.success) {
            const out = res.stdout.toLowerCase();
            if (out.includes('batch')) {
                mode = 'batch';
            } else if (out.includes('streaming')) {
                mode = 'streaming';
            }
        }
        this._currentMode = mode;
        this._modeStatusLabel.text = `(${mode})`;

        if (mode === 'batch') {
            this._batchModeBtn.add_style_class_name('active');
            this._streamingModeBtn.remove_style_class_name('active');
        } else if (mode === 'streaming') {
            this._streamingModeBtn.add_style_class_name('active');
            this._batchModeBtn.remove_style_class_name('active');
        } else {
            this._batchModeBtn.remove_style_class_name('active');
            this._streamingModeBtn.remove_style_class_name('active');
        }
    }

    async _switchMode(targetMode) {
        this._modeStatusLabel.text = `(switching to ${targetMode}...)`;
        const res = await runHarnez(['tools', 'voice-input', 'mode', targetMode]);
        if (!res.success) {
            console.error(`[HARNEZ_VOICE_INPUT_EXT] Mode switch failed:`, res.stderr);
            Main.notify('Voice Input Mode', res.stderr || 'Failed to switch mode');
        }
        await this._refreshMode();
    }

    async _refreshConfig() {
        const res = await runHarnez(['tools', 'voice-input', 'config', 'get', 'type-delay-ms']);
        let ms = 0;
        if (res.success) {
            const parsed = parseInt(res.stdout, 10);
            if (!isNaN(parsed) && parsed >= 0) {
                ms = parsed;
            }
        }
        this._typeDelayMs = ms;
        this._delayValueLabel.text = `${ms} ms`;
        // Map 0-100 ms to slider 0.0-1.0
        const sliderVal = Math.min(Math.max(ms / 100.0, 0.0), 1.0);
        this._delaySlider.value = sliderVal;
    }

    _onSliderMoved() {
        const ms = Math.round(this._delaySlider.value * 100);
        this._delayValueLabel.text = `${ms} ms`;
    }

    async _onSliderCommitted() {
        const ms = Math.round(this._delaySlider.value * 100);
        await this._setDelayMs(ms);
    }

    async _setDelayMs(ms) {
        this._typeDelayMs = ms;
        this._delayValueLabel.text = `${ms} ms`;
        this._delaySlider.value = Math.min(Math.max(ms / 100.0, 0.0), 1.0);

        const res = await runHarnez(['tools', 'voice-input', 'config', 'set', 'type-delay-ms', String(ms)]);
        if (!res.success) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Failed to set type-delay-ms:', res.stderr);
        }
    }

    async _refreshHistory() {
        this._historyListBox.destroy_all_children();

        let entries = [];
        const res = await runHarnez(['tools', 'voice-input', 'history', 'list', '--format', 'json']);
        if (res.success && res.stdout) {
            try {
                entries = JSON.parse(res.stdout);
            } catch (e) {
                console.warn('[HARNEZ_VOICE_INPUT_EXT] Failed to parse history JSON from CLI:', e);
            }
        }

        // Direct file fallback if CLI output was empty/failed
        if (!entries || entries.length === 0) {
            entries = this._readHistoryDirectly();
        }

        if (!entries || entries.length === 0) {
            const emptyLabel = new St.Label({
                text: 'No recent dictations recorded.',
                style_class: 'voice-input-note',
                x_align: Clutter.ActorAlign.CENTER,
            });
            this._historyListBox.add_child(emptyLabel);
            return;
        }

        entries.forEach(entry => {
            const itemBox = new St.BoxLayout({
                vertical: true,
                style_class: 'voice-input-history-item',
                x_expand: true,
            });

            const topRow = new St.BoxLayout({ x_expand: true });
            
            let timeStr = '';
            if (entry.time) {
                try {
                    const d = new Date(entry.time);
                    timeStr = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
                } catch (_) {
                    timeStr = String(entry.time);
                }
            }

            const timeLabel = new St.Label({
                text: timeStr || entry.id || '',
                style_class: 'voice-input-history-time',
                x_expand: true,
                y_align: Clutter.ActorAlign.CENTER,
            });

            const copyBtn = new St.Button({
                label: 'Copy',
                style_class: 'voice-input-action-btn',
                y_align: Clutter.ActorAlign.CENTER,
            });
            copyBtn.connect('clicked', () => this._onCopyEntry(entry.id, entry.text));

            const retypeBtn = new St.Button({
                label: 'Retype',
                style_class: 'voice-input-action-btn retype-btn',
                y_align: Clutter.ActorAlign.CENTER,
            });
            retypeBtn.connect('clicked', () => this._onRetypeEntry(entry.id, entry.text));

            topRow.add_child(timeLabel);
            topRow.add_child(copyBtn);
            topRow.add_child(retypeBtn);

            const textLabel = new St.Label({
                text: entry.text || '',
                style_class: 'voice-input-history-text',
                x_expand: true,
            });
            textLabel.clutter_text.set_line_wrap(true);
            textLabel.clutter_text.set_ellipsize(3); // PANGO_ELLIPSIZE_END

            itemBox.add_child(topRow);
            itemBox.add_child(textLabel);
            this._historyListBox.add_child(itemBox);
        });
    }

    _readHistoryDirectly() {
        const historyFile = GLib.build_filenamev([
            GLib.get_home_dir(),
            '.local', 'share', 'harnez', 'voice-input', 'history.jsonl',
        ]);
        if (!GLib.file_test(historyFile, GLib.FileTest.EXISTS)) {
            return [];
        }
        try {
            const [ok, contents] = GLib.file_get_contents(historyFile);
            if (!ok) return [];
            const text = new TextDecoder().decode(contents);
            const lines = text.trim().split('\n').filter(Boolean);
            const entries = [];
            for (let i = lines.length - 1; i >= 0; i--) {
                try {
                    entries.push(JSON.parse(lines[i]));
                } catch (_) {}
            }
            return entries;
        } catch (e) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Error reading history directly:', e);
            return [];
        }
    }

    async _onCopyEntry(id, text) {
        console.log(`[HARNEZ_VOICE_INPUT_EXT] Copying entry ${id}`);
        const res = await runHarnez(['tools', 'voice-input', 'history', 'copy', id]);
        if (res.success) {
            Main.notify('Voice Input', 'Transcript copied to clipboard');
        } else {
            // Fallback via St.Clipboard
            St.Clipboard.get_default().set_text(St.ClipboardType.CLIPBOARD, text || '');
            Main.notify('Voice Input', 'Transcript copied to clipboard');
        }
    }

    _onRetypeEntry(id, text) {
        const win = this._capturedWindow;
        this._indicator.menu.close();

        if (!win) {
            console.warn('[HARNEZ_VOICE_INPUT_EXT] No captured window to retype into');
            Main.notify('Voice Input Retype', 'No active application window detected prior to menu open.');
            return;
        }

        const executeRetype = async () => {
            const currentWin = global.display.get_focus_window();
            const currentTitle = currentWin ? currentWin.get_title() : 'none';
            console.log(`[HARNEZ_VOICE_INPUT_EXT] Retype trigger: target window "${win.get_title()}", current focused "${currentTitle}"`);
            
            try {
                const res = await runHarnez(['tools', 'voice-input', 'history', 'retype', id]);
                if (!res.success) {
                    console.error('[HARNEZ_VOICE_INPUT_EXT] Retype command error:', res.stderr);
                    Main.notify('Voice Input Retype Failed', res.stderr || 'Command error');
                }
            } catch (err) {
                console.error('[HARNEZ_VOICE_INPUT_EXT] Retype execution exception:', err);
            }
        };

        if (global.display.get_focus_window() === win) {
            executeRetype();
            return;
        }

        let signalId = 0;
        let timeoutId = 0;

        const cleanup = () => {
            if (signalId && global.display) {
                global.display.disconnect(signalId);
                signalId = 0;
            }
            if (timeoutId) {
                GLib.source_remove(timeoutId);
                timeoutId = 0;
            }
        };

        signalId = global.display.connect('notify::focus-window', () => {
            if (global.display.get_focus_window() === win) {
                cleanup();
                executeRetype();
            }
        });

        // Settle fallback timeout (800ms)
        timeoutId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 800, () => {
            cleanup();
            executeRetype();
            return GLib.SOURCE_REMOVE;
        });

        try {
            win.activate(global.get_current_time());
        } catch (err) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Failed to activate window:', err);
            cleanup();
            executeRetype();
        }
    }

    async _onClearHistory() {
        const res = await runHarnez(['tools', 'voice-input', 'history', 'clear']);
        if (!res.success) {
            console.error('[HARNEZ_VOICE_INPUT_EXT] Failed to clear history:', res.stderr);
        }
        await this._refreshHistory();
    }
}
