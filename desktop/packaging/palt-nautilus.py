# PALT for Nautilus: right-click files → "Send with PALT" → a paired device.
#
# Installed to /usr/share/nautilus-python/extensions/ by the .deb (needs the
# python3-nautilus package). Lists the devices this computer has been paired
# with and hands the files to the running PALT ("palt --send").

import json
import os
import shutil
import subprocess
from urllib.parse import unquote, urlparse

from gi.repository import GObject, Nautilus

DEVICES = os.path.join(
    os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config"), "palt", "devices.json"
)


def _devices():
    try:
        with open(DEVICES) as f:
            known = json.load(f)
    except (OSError, ValueError):
        return []
    # Most recently seen first.
    items = sorted(known.items(), key=lambda kv: kv[1].get("lastSeen", ""), reverse=True)
    return [(dev_id, d.get("name") or dev_id) for dev_id, d in items]


def _palt():
    return shutil.which("palt") or "/usr/local/bin/palt"


class PaltMenu(GObject.GObject, Nautilus.MenuProvider):
    def _send(self, _item, dev_id, paths):
        subprocess.Popen([_palt(), "--send", dev_id, *paths], start_new_session=True)

    def get_file_items(self, *args):
        files = args[-1]
        paths = []
        for f in files:
            uri = f.get_uri()
            if not uri.startswith("file://") or f.is_directory():
                return []
            paths.append(unquote(urlparse(uri).path))
        devices = _devices()
        if not paths or not devices:
            return []

        top = Nautilus.MenuItem(name="PaltMenu::Top", label="Send with PALT")
        sub = Nautilus.Menu()
        top.set_submenu(sub)
        for dev_id, name in devices:
            item = Nautilus.MenuItem(name="PaltMenu::" + dev_id, label=name)
            item.connect("activate", self._send, dev_id, paths)
            sub.append_item(item)
        return [top]
