// PALT clipboard helper: exposes the clipboard to PALT on the session bus.
//
//   org.palt.Clipboard /org/palt/Clipboard
//     SetText(s)        put text on the clipboard
//     signal Changed(s) the clipboard now holds this text
//
// GNOME only lets the focused app use the clipboard; PALT runs in the
// background, so the shell (which may) does it on PALT's behalf.

import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import Meta from 'gi://Meta';
import St from 'gi://St';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const IFACE = `<node>
  <interface name="org.palt.Clipboard">
    <method name="SetText"><arg type="s" direction="in" name="text"/></method>
    <signal name="Changed"><arg type="s" name="text"/></signal>
  </interface>
</node>`;

const MAX_TEXT = 1024 * 1024;

export default class PaltClipboard extends Extension {
    enable() {
        this._clipboard = St.Clipboard.get_default();
        this._dbus = Gio.DBusExportedObject.wrapJSObject(IFACE, this);
        this._dbus.export(Gio.DBus.session, '/org/palt/Clipboard');
        this._name = Gio.bus_own_name_on_connection(
            Gio.DBus.session, 'org.palt.Clipboard', Gio.BusNameOwnerFlags.NONE, null, null);

        this._selection = global.display.get_selection();
        this._ownerChanged = this._selection.connect('owner-changed', (_sel, type) => {
            if (type !== Meta.SelectionType.SELECTION_CLIPBOARD)
                return;
            this._clipboard.get_text(St.ClipboardType.CLIPBOARD, (_c, text) => {
                if (text && text.length <= MAX_TEXT)
                    this._dbus.emit_signal('Changed', new GLib.Variant('(s)', [text]));
            });
        });
    }

    disable() {
        this._selection?.disconnect(this._ownerChanged);
        this._dbus?.unexport();
        if (this._name)
            Gio.bus_unown_name(this._name);
        this._selection = this._dbus = this._name = this._clipboard = null;
    }

    SetText(text) {
        this._clipboard.set_text(St.ClipboardType.CLIPBOARD, text);
    }
}
