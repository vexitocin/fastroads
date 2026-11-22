#!/usr/bin/env python3
"""Fastroads server + WebView + Discord RPC (safe + logged version)"""

import functools
import http.server
import math
import socket
import socketserver
import threading
import time
import traceback
import urllib.parse
from datetime import datetime
from pathlib import Path

try:
    import webview
except ImportError:
    raise SystemExit("pywebview missing: pip install pywebview")

try:
    from pypresence import Presence
except ImportError:
    Presence = None


# ----------------------------
# LOGGING SYSTEM
# ----------------------------

ROOT = Path(__file__).resolve().parent
LOG_FILE = ROOT / "fastroads.log"

def log(msg: str):
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(f"[{datetime.now().isoformat()}] {msg}\n")

def log_exception():
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write("\n" + "=" * 50 + "\n")
        f.write(traceback.format_exc())
        f.write("=" * 50 + "\n\n")


# ----------------------------
# CONFIG
# ----------------------------

HOST = "127.0.0.1"
ICO_PATH = ROOT / "favicon_circle.ico"
DISCORD_CLIENT_ID = "1499447238923915274"
RPC_POLLING_SCRIPT = """
(() => {
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
# RPC SYSTEM (Discord + arRPC compatible)
# ----------------------------

def start_rpc():
    if Presence is None:
        log("[RPC] pypresence not installed")
        return None

    try:
        rpc = Presence(DISCORD_CLIENT_ID)
        rpc.connect()

        rpc.update(
            details="Playing Fastroads",
            state="In Game",
            large_image="logo",
            large_text="Fastroads",
            start=int(time.time())
        )

        log("[RPC] Connected successfully")
        return rpc

    except Exception:
        log("[RPC] Failed to connect")
        log_exception()
        return None


class RPCBridge:
    def __init__(self, rpc):
        self.rpc = rpc
        self.started_at = int(time.time())
        self.lock = threading.Lock()
        self.last_activity = None

    def update_activity(self, paused, speed_mps, distance_m):
        if self.rpc is None:
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
            try:
                self.rpc.update(
                    details=details,
                    state=state,
                    large_image="logo",
                    large_text="Fastroads",
                    start=self.started_at,
                )
            except Exception:
                log("[RPC] Failed to update activity")
                log_exception()
                return False
            self.last_activity = activity
        return True

    def close(self):
        if self.rpc is None:
            return
        with self.lock:
            self.rpc.close()


def install_rpc_bridge(window):
    def on_loaded():
        try:
            window.evaluate_js(RPC_POLLING_SCRIPT)
        except Exception:
            log("[RPC] Failed to install browser telemetry bridge")
            log_exception()

    window.events.loaded += on_loaded


# ----------------------------
# MAIN APP
# ----------------------------

def main():
    log("=== Fastroads starting ===")

    try:
        port = find_free_port()
        server = start_server(port)
        url = f"http://{HOST}:{port}/"

        log(f"Server running at {url}")

        rpc_bridge = RPCBridge(start_rpc())

        log("Launching WebView")

        window = webview.create_window(
            "f a s t r o a d s",
            url,
            width=1280,
            height=800,
            fullscreen=True,
            js_api=rpc_bridge,
        )
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
