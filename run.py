#!/usr/bin/env python3
"""Fastroads server + WebView + Discord RPC (safe + logged version)"""

import argparse
import functools
import http.server
import json
import math
import os
import queue
import socket
import socketserver
import sys
import threading
import time
import traceback
import urllib.parse
from datetime import datetime
from pathlib import Path

from updater import auto_update_if_available

try:
    import webview
except ImportError:
    webview = None

try:
    from pypresence import Presence
except ImportError:
    Presence = None


# ----------------------------
# LOGGING SYSTEM
# ----------------------------

ROOT = Path(__file__).resolve().parent
LOG_FILE = ROOT / "fastroads.log"
LOG_LOCK = threading.Lock()

def log(msg: str):
    line = f"[{datetime.now().isoformat()}] {msg}"
    with LOG_LOCK:
        print(line, flush=True)
        with open(LOG_FILE, "a", encoding="utf-8") as f:
            f.write(line + "\n")

def log_exception():
    details = traceback.format_exc()
    with LOG_LOCK:
        print(details, end="", flush=True)
        with open(LOG_FILE, "a", encoding="utf-8") as f:
            f.write(details)


# ----------------------------
# CONFIG
# ----------------------------

HOST = "127.0.0.1"
ICO_PATH = ROOT / "favicon_circle.ico"
DISCORD_CLIENT_ID = "1499447238923915274"
RPC_POLLING_SCRIPT = """
(() => {
    if (window.__fastroadsRPCPolling) return;
    window.__fastroadsRPCPolling = true;

    const publishActivity = () => {
        const api = window.pywebview && window.pywebview.api;
        const game = window.__fastroadsGame;
        const telemetry = window.__fastroadsTelemetry;
        if (!api || !game || !telemetry) return;

        const speed = Number(telemetry.speed);
        const distance = Number(telemetry.dist);
        if (!Number.isFinite(speed) || !Number.isFinite(distance)) return;

        api.update_activity(Boolean(game.paused), speed, distance)
            .catch(error => console.error("Discord RPC update failed:", error));
    };

    window.setInterval(publishActivity, 2000);
    publishActivity();
})();
"""


def load_icon_path():
    return str(ICO_PATH) if ICO_PATH.exists() else None


def launcher_settings_path():
    if sys.platform == "win32":
        base = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
    elif sys.platform == "darwin":
        base = Path.home() / "Library" / "Application Support"
    else:
        base = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
    return base / "fastroads" / "launcher-settings.json"


def load_launcher_settings():
    settings = {"rpc_enabled": True, "fullscreen": True}
    path = launcher_settings_path()
    try:
        if path.exists():
            saved = json.loads(path.read_text(encoding="utf-8"))
            if not isinstance(saved, dict):
                log(f"[Settings] Ignoring invalid launcher settings in {path}")
                return settings
            for key in settings:
                if key in saved and isinstance(saved[key], bool):
                    settings[key] = saved[key]
                elif key in saved:
                    log(f"[Settings] Ignoring invalid {key} value")
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        log(f"[Settings] Could not load {path}: {error}")
    return settings


def save_launcher_settings(settings):
    path = launcher_settings_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(settings, indent=2) + "\n", encoding="utf-8")
    except OSError as error:
        log(f"[Settings] Could not save {path}: {error}")
        return False
    return True


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description="Launch Fastroads.")
    rpc_options = parser.add_mutually_exclusive_group()
    rpc_options.add_argument(
        "--rpc", dest="rpc_enabled", action="store_true",
        help="enable Discord Rich Presence (default)",
    )
    rpc_options.add_argument(
        "--no-rpc", dest="rpc_enabled", action="store_false",
        help="disable Discord Rich Presence",
    )
    display_options = parser.add_mutually_exclusive_group()
    display_options.add_argument(
        "--fullscreen", dest="fullscreen", action="store_true",
        help="launch fullscreen (default)",
    )
    display_options.add_argument(
        "--windowed", dest="fullscreen", action="store_false",
        help="launch in a resizable window",
    )
    parser.set_defaults(rpc_enabled=None, fullscreen=None)
    return parser.parse_args(argv)


# ----------------------------
# HTTP SERVER
# ----------------------------

class GameRequestHandler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self, path: str) -> str:
        parsed = urllib.parse.urlparse(path)
        if parsed.path == "/null":
            path = "/null.html"
        return super().translate_path(path)


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True


def find_free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind((HOST, 0))
        return s.getsockname()[1]


def start_server(port: int):
    handler = functools.partial(GameRequestHandler, directory=str(ROOT))
    server = ThreadingHTTPServer((HOST, port), handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server


# ----------------------------
# DISCORD RICH PRESENCE
# ----------------------------
class RPCBridge:
    def __init__(self, settings):
        self.settings = dict(settings)
        self.enabled = self.settings["rpc_enabled"] and Presence is not None
        self.fullscreen = self.settings["fullscreen"]
        self.window = None
        self.started_at = int(time.time())
        self.lock = threading.Lock()
        self.last_activity = None
        self.updates = queue.Queue(maxsize=1)
        self.stop_event = threading.Event()
        self.worker = None

        if not self.settings["rpc_enabled"]:
            log("[RPC] Disabled in launcher settings")
        elif Presence is None:
            log("[RPC] pypresence is not installed; RPC is disabled")
        else:
            self._start_worker()

    def _start_worker(self):
        self.stop_event = threading.Event()
        self.updates = queue.Queue(maxsize=1)
        self.last_activity = None
        self.worker = threading.Thread(target=self._run, daemon=True)
        self.worker.start()

    def get_launcher_settings(self):
        with self.lock:
            return dict(
                rpc_enabled=self.enabled,
                fullscreen=self.fullscreen,
            )

    def set_rpc_enabled(self, enabled):
        if not isinstance(enabled, bool):
            log("[Settings] Ignoring invalid RPC setting")
            return False
        if enabled and Presence is None:
            log("[RPC] Cannot enable Rich Presence: pypresence is not installed")
            return False

        with self.lock:
            if enabled == self.enabled:
                return True
            self.enabled = enabled
            self.settings["rpc_enabled"] = enabled

            if enabled:
                if self.worker is not None and self.worker.is_alive():
                    log("[RPC] Cannot re-enable while the previous connection is closing")
                    self.enabled = False
                    self.settings["rpc_enabled"] = False
                    return False
                self._start_worker()
                log("[RPC] Enabled from Settings")
                save_launcher_settings(self.settings)
                return True

            worker = self.worker
            self.stop_event.set()
            try:
                self.updates.put_nowait(None)
            except queue.Full:
                try:
                    self.updates.get_nowait()
                except queue.Empty:
                    pass
                self.updates.put_nowait(None)

        if worker is not None:
            worker.join(timeout=6)
            if worker.is_alive():
                log("[RPC] Worker is still shutting down")
                return False
        with self.lock:
            self.worker = None
            self.last_activity = None
            log("[RPC] Disabled from Settings")
            save_launcher_settings(self.settings)
            return True

    def set_fullscreen(self, fullscreen):
        if not isinstance(fullscreen, bool):
            log("[Settings] Ignoring invalid display mode")
            return False
        with self.lock:
            if fullscreen == self.fullscreen:
                return True
            window = self.window
            if window is None:
                log("[Settings] Display mode cannot change before the window is ready")
                return False
            try:
                window.toggle_fullscreen()
            except Exception as error:
                log(f"[Settings] Could not change display mode: {error}")
                log_exception()
                return False
            self.fullscreen = fullscreen
            self.settings["fullscreen"] = fullscreen
            log(f"[Settings] Display mode set to {'fullscreen' if fullscreen else 'windowed'}")
            save_launcher_settings(self.settings)
            return True

    def update_activity(self, paused, speed_mps, distance_m):
        if not self.enabled:
            return False
        if not isinstance(paused, bool):
            log("[RPC] Ignoring activity update with invalid pause state")
            return False
        try:
            speed_mps = float(speed_mps)
            distance_m = float(distance_m)
        except (TypeError, ValueError):
            log("[RPC] Ignoring activity update with invalid telemetry")
            return False
        if not math.isfinite(speed_mps) or not math.isfinite(distance_m):
            log("[RPC] Ignoring activity update with non-finite telemetry")
            return False

        speed_mph = abs(speed_mps) * 2.2369362920544
        distance_miles = abs(distance_m) / 1609.344
        details = "Paused in Fastroads" if paused else "Driving in Fastroads"
        state = f"{speed_mph:.0f} mph | {distance_miles:.1f} mi"
        activity = (details, state)

        with self.lock:
            if activity == self.last_activity:
                return True
            if self.updates.full():
                try:
                    self.updates.get_nowait()
                except queue.Empty:
                    pass
            self.updates.put_nowait(activity)
            self.last_activity = activity
        return True

    def _run(self):
        while not self.stop_event.is_set():
            rpc = None
            activity = None
            try:
                log("[RPC] Connecting to Discord...")
                rpc = Presence(
                    DISCORD_CLIENT_ID,
                    connection_timeout=5,
                    response_timeout=5,
                )
                rpc.connect()
                log("[RPC] Connected to Discord")

                while not self.stop_event.is_set():
                    try:
                        activity = self.updates.get(timeout=0.5)
                    except queue.Empty:
                        continue
                    if activity is None:
                        break
                    details, state = activity
                    rpc.update(
                        details=details,
                        state=state,
                        large_image="logo",
                        large_text="Fastroads",
                        start=self.started_at,
                    )
                    log(f"[RPC] Activity updated: {details} | {state}")
                    activity = None
            except Exception as error:
                log(f"[RPC] Connection or update failed: {error}")
                log_exception()
                if activity is not None:
                    with self.lock:
                        if self.last_activity == activity:
                            self.last_activity = None
                        if not self.updates.full():
                            self.updates.put_nowait(activity)
            finally:
                if rpc is not None:
                    try:
                        rpc.close()
                    except Exception as error:
                        log(f"[RPC] Failed to close connection: {error}")
                        log_exception()

            if not self.stop_event.is_set():
                log("[RPC] Retrying connection in 5 seconds")
                self.stop_event.wait(5)

    def close(self):
        if self.worker is None:
            return
        self.stop_event.set()
        try:
            self.updates.put_nowait(None)
        except queue.Full:
            try:
                self.updates.get_nowait()
            except queue.Empty:
                pass
            self.updates.put_nowait(None)
        self.worker.join(timeout=6)
        if self.worker.is_alive():
            log("[RPC] Worker is still shutting down")
        else:
            self.worker = None


def install_rpc_bridge(window):
    def on_loaded():
        try:
            window.evaluate_js(RPC_POLLING_SCRIPT)
            log("[RPC] Browser telemetry bridge installed")
        except Exception:
            log("[RPC] Failed to install browser telemetry bridge")
            log_exception()

    window.events.loaded += on_loaded


# ----------------------------
# MAIN APP
# ----------------------------

def main():
    args = parse_args()
    if auto_update_if_available(sys.argv[1:], log):
        log("[Update] Closing this version; the updated release will start next")
        return
    if webview is None:
        raise SystemExit("pywebview missing: pip install pywebview")
    settings = load_launcher_settings()
    if args.rpc_enabled is not None:
        settings["rpc_enabled"] = args.rpc_enabled
    if args.fullscreen is not None:
        settings["fullscreen"] = args.fullscreen
    log("=== Fastroads starting ===")
    log(
        f"Options: Discord RPC {'enabled' if settings['rpc_enabled'] else 'disabled'}, "
        f"{'fullscreen' if settings['fullscreen'] else 'windowed'}"
    )

    try:
        port = find_free_port()
        server = start_server(port)
        url = f"http://{HOST}:{port}/"

        log(f"Server running at {url}")

        rpc_bridge = RPCBridge(settings)

        log("Launching WebView")

        window = webview.create_window(
            "f a s t r o a d s",
            url,
            width=1280,
            height=800,
            fullscreen=settings["fullscreen"],
            js_api=rpc_bridge,
        )
        rpc_bridge.window = window
        install_rpc_bridge(window)

        icon = load_icon_path()

        webview.start(gui="edgechromium", icon=icon)

        try:
            rpc_bridge.close()
        except Exception:
            log("[RPC] Failed to close connection")
            log_exception()

        server.shutdown()
        server.server_close()

        log("Clean shutdown complete")

    except Exception:
        log_exception()
        raise


if __name__ == "__main__":
    main()
