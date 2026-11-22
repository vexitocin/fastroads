"""Automatic updater for packaged Windows releases."""

import hashlib
import http.client
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from pathlib import Path


RELEASE_API = "https://api.github.com/repos/vexitocin/fastroads/releases/latest"
CHANNEL_ASSETS = {
    "normal": "fastroads-normal.zip",
    "debug": "fastroads-debug.zip",
    "onefile": "fastroads-onefile.zip",
}
MAX_ARCHIVE_SIZE = 512 * 1024 * 1024
MAX_EXTRACTED_SIZE = 1024 * 1024 * 1024
MAX_ARCHIVE_FILES = 100_000
HELPER_START_TIMEOUT = 30
USER_AGENT = "Fastroads-Updater"


def parse_version(version):
    if not isinstance(version, str):
        raise ValueError(f"Unsupported release version: {version!r}")
    match = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", version)
    if match is None:
        raise ValueError(f"Unsupported release version: {version!r}")
    return tuple(int(part) for part in match.groups())


def _read_install_metadata(install_root):
    metadata_path = install_root / "fastroads-update.json"
    try:
        metadata = json.loads(metadata_path.read_text(encoding="utf-8-sig"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError(f"Could not read {metadata_path}: {error}") from error

    if not isinstance(metadata, dict):
        raise ValueError("Update metadata must be a JSON object")
    version = metadata.get("version")
    channel = metadata.get("channel")
    parse_version(version)
    if not isinstance(channel, str) or channel not in CHANNEL_ASSETS:
        raise ValueError(f"Unsupported release channel: {channel!r}")
    return version, channel


def _request_json(url):
    request = urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": USER_AGENT,
        },
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.load(response)


def _download_asset(url, destination, expected_size, expected_digest):
    if (
        type(expected_size) is not int
        or expected_size < 1
        or expected_size > MAX_ARCHIVE_SIZE
    ):
        raise ValueError("Release asset has an invalid or excessive size")
    if not isinstance(expected_digest, str) or not re.fullmatch(
        r"sha256:[0-9a-f]{64}", expected_digest
    ):
        raise ValueError("Release asset does not have a valid SHA-256 digest")

    digest = hashlib.sha256()
    downloaded = 0
    request = urllib.request.Request(
        url, headers={"User-Agent": USER_AGENT}
    )
    with urllib.request.urlopen(request, timeout=15) as response:
        with destination.open("wb") as archive:
            while True:
                block = response.read(1024 * 1024)
                if not block:
                    break
                downloaded += len(block)
                if downloaded > expected_size or downloaded > MAX_ARCHIVE_SIZE:
                    raise ValueError("Downloaded release asset exceeds its size limit")
                digest.update(block)
                archive.write(block)

    if downloaded != expected_size:
        raise ValueError("Downloaded release asset size does not match GitHub metadata")
    if f"sha256:{digest.hexdigest()}" != expected_digest:
        raise ValueError("Downloaded release asset SHA-256 does not match")


def _extract_release_zip(archive_path, destination, expected_version, channel):
    total_size = 0
    with zipfile.ZipFile(archive_path) as archive:
        members = archive.infolist()
        if len(members) > MAX_ARCHIVE_FILES:
            raise ValueError("Release archive contains too many files")

        root = destination.resolve()
        for member in members:
            name = member.filename
            if not name or "\\" in name or ":" in name:
                raise ValueError("Release archive contains an invalid path")
            relative = Path(name)
            if relative.is_absolute() or ".." in relative.parts:
                raise ValueError("Release archive contains a path outside its root")
            target = (destination / relative).resolve()
            if os.path.commonpath((str(root), str(target))) != str(root):
                raise ValueError("Release archive contains a path outside its root")
            if stat.S_ISLNK(member.external_attr >> 16):
                raise ValueError("Release archive contains a symbolic link")
            total_size += member.file_size
            if total_size > MAX_EXTRACTED_SIZE:
                raise ValueError("Release archive exceeds its extracted size limit")

        for member in members:
            target = destination / member.filename
            if member.is_dir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            target.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(member) as source, target.open("wb") as output:
                shutil.copyfileobj(source, output)

    metadata_path = destination / "fastroads-update.json"
    try:
        metadata = json.loads(metadata_path.read_text(encoding="utf-8-sig"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ValueError("Release archive has invalid update metadata") from error
    if (
        not isinstance(metadata, dict)
        or metadata.get("version") != expected_version
        or metadata.get("channel") != channel
    ):
        raise ValueError("Release archive metadata does not match the selected release")
    if not (destination / "fastroads.exe").is_file():
        raise ValueError("Release archive does not contain fastroads.exe")
    if channel in ("normal", "debug") and not (
        destination / "_internal"
    ).is_dir():
        raise ValueError("Release archive does not contain the app files")


def _ps_quote(value):
    return "'" + str(value).replace("'", "''") + "'"


def _updater_script(
    install_root, package_root, stage_root, process_id, channel, launch_args
):
    backup_root = stage_root / "backup"
    status_path = stage_root / "helper-started"
    log_path = install_root / "fastroads-update.log"
    managed_items = ["fastroads.exe", "fastroads-update.json"]
    if channel != "onefile":
        managed_items.append("_internal")

    item_list = "@(" + ", ".join(_ps_quote(item) for item in managed_items) + ")"
    arg_list = "@(" + ", ".join(_ps_quote(arg) for arg in launch_args) + ")"
    return f"""$ErrorActionPreference = 'Stop'
$InstallRoot = {_ps_quote(install_root)}
$PackageRoot = {_ps_quote(package_root)}
$StageRoot = {_ps_quote(stage_root)}
$BackupRoot = {_ps_quote(backup_root)}
$StatusPath = {_ps_quote(status_path)}
$LogPath = {_ps_quote(log_path)}
$ProcessIdToWait = {int(process_id)}
$ManagedItems = {item_list}
$LaunchArgs = {arg_list}
function Write-UpdateLog($Message) {{
    Add-Content -LiteralPath $LogPath -Value $Message -ErrorAction SilentlyContinue
}}
Set-Content -LiteralPath $StatusPath -Value 'ready' -NoNewline
while (Get-Process -Id $ProcessIdToWait -ErrorAction SilentlyContinue) {{
    Start-Sleep -Milliseconds 250
}}
$UpdateSucceeded = $false
$RollbackSucceeded = $false
try {{
    New-Item -ItemType Directory -Path $BackupRoot | Out-Null
    foreach ($item in $ManagedItems) {{
        $oldPath = Join-Path $InstallRoot $item
        if (Test-Path -LiteralPath $oldPath) {{
            Move-Item -LiteralPath $oldPath -Destination $BackupRoot
        }}
    }}
    foreach ($item in $ManagedItems) {{
        $newPath = Join-Path $PackageRoot $item
        if (Test-Path -LiteralPath $newPath) {{
            Move-Item -LiteralPath $newPath -Destination $InstallRoot
        }}
    }}
    $Executable = Join-Path $InstallRoot 'fastroads.exe'
    if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {{
        throw 'Updated executable is missing'
    }}
    if ($LaunchArgs.Count -gt 0) {{
        Start-Process -FilePath $Executable -ArgumentList $LaunchArgs
    }} else {{
        Start-Process -FilePath $Executable
    }}
    $UpdateSucceeded = $true
}} catch {{
    Write-UpdateLog "[Update] Installation failed: $_"
    try {{
        foreach ($item in $ManagedItems) {{
            $oldPath = Join-Path $InstallRoot $item
            $backupPath = Join-Path $BackupRoot $item
            if (Test-Path -LiteralPath $backupPath) {{
                if (Test-Path -LiteralPath $oldPath) {{
                    Remove-Item -LiteralPath $oldPath -Recurse -Force
                }}
                Move-Item -LiteralPath $backupPath -Destination $InstallRoot
            }}
        }}
        $Executable = Join-Path $InstallRoot 'fastroads.exe'
        if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {{
            throw 'Previous executable could not be restored'
        }}
        if ($LaunchArgs.Count -gt 0) {{
            Start-Process -FilePath $Executable -ArgumentList $LaunchArgs
        }} else {{
            Start-Process -FilePath $Executable
        }}
        $RollbackSucceeded = $true
    }} catch {{
        Write-UpdateLog "[Update] Rollback failed: $_"
    }}
}}
if ($UpdateSucceeded) {{
    Write-UpdateLog '[Update] Update installed and relaunched.'
}}
if ($UpdateSucceeded -or $RollbackSucceeded) {{
    Remove-Item -LiteralPath $BackupRoot -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
}}
Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
"""


def _start_update_helper(
    install_root, package_root, stage_root, channel, launch_args, log
):
    script_path = None
    process = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8-sig",
            suffix=".ps1",
            prefix="fastroads-update-",
            delete=False,
        ) as script:
            script.write(
                _updater_script(
                    install_root,
                    package_root,
                    stage_root,
                    os.getpid(),
                    channel,
                    launch_args,
                )
            )
            script_path = Path(script.name)

        update_log = install_root / "fastroads-update.log"
        with update_log.open("a", encoding="utf-8") as output:
            process = subprocess.Popen(
                [
                    "powershell.exe",
                    "-NoProfile",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-File",
                    str(script_path),
                ],
                stdout=output,
                stderr=subprocess.STDOUT,
                creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
            )

        status_path = stage_root / "helper-started"
        deadline = time.monotonic() + HELPER_START_TIMEOUT
        while time.monotonic() < deadline:
            if status_path.is_file():
                return True
            if process.poll() is not None:
                break
            time.sleep(0.1)
        exit_code = process.poll()
        if exit_code is None:
            log(
                f"[Update] Update helper did not signal startup within "
                f"{HELPER_START_TIMEOUT} seconds; stopping it"
            )
        else:
            log(
                f"[Update] Update helper exited with status {exit_code} "
                "before signaling startup"
            )
        try:
            with update_log.open("r", encoding="utf-8", errors="replace") as output:
                details = "".join(output.readlines()[-8:]).strip()
            if details:
                log(f"[Update] Helper output: {details}")
        except OSError as error:
            log(f"[Update] Could not read helper output: {error}")
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=2)
        return False
    except (OSError, subprocess.SubprocessError) as error:
        log(f"[Update] Could not start the update helper: {error}")
        return False
    finally:
        if script_path is not None:
            try:
                script_path.unlink(missing_ok=True)
            except OSError as error:
                log(f"[Update] Could not remove temporary helper script: {error}")


def update_and_relaunch(install_root, launch_args, log):
    """Install a newer packaged release and hand off to a detached helper."""
    install_root = Path(install_root).resolve()
    stage_root = None
    handed_off = False
    try:
        current_version, channel = _read_install_metadata(install_root)
        log(f"[Update] Checking for updates from {current_version}...")
        release = _request_json(RELEASE_API)
        if not isinstance(release, dict):
            raise ValueError("Latest release response is not a JSON object")
        if release.get("draft") or release.get("prerelease"):
            log("[Update] Skipping a draft or prerelease build")
            return False
        latest_version = release.get("tag_name")
        if not isinstance(latest_version, str):
            raise ValueError("Latest release has no valid tag")
        if parse_version(latest_version) <= parse_version(current_version):
            log("[Update] Already using the latest release")
            return False

        asset_name = CHANNEL_ASSETS[channel]
        assets = release.get("assets")
        if not isinstance(assets, list):
            raise ValueError("Latest release has no valid assets list")
        asset = next(
            (
                item
                for item in assets
                if isinstance(item, dict) and item.get("name") == asset_name
            ),
            None,
        )
        if asset is None:
            raise ValueError(f"Latest release does not contain {asset_name}")
        download_url = (
            "https://github.com/vexitocin/fastroads/releases/download/"
            f"{urllib.parse.quote(latest_version, safe='')}/"
            f"{urllib.parse.quote(asset_name, safe='')}"
        )

        stage_root = Path(
            tempfile.mkdtemp(
                prefix=".fastroads-update-", dir=str(install_root.parent)
            )
        )
        archive_path = stage_root / asset_name
        package_root = stage_root / "package"
        package_root.mkdir()
        _download_asset(
            download_url,
            archive_path,
            asset.get("size"),
            asset.get("digest"),
        )
        _extract_release_zip(
            archive_path, package_root, latest_version, channel
        )
        log(f"[Update] Installing {latest_version}...")

        if _start_update_helper(
            install_root,
            package_root,
            stage_root,
            channel,
            launch_args,
            log,
        ):
            handed_off = True
            return True
        return False
    except (
        OSError,
        urllib.error.URLError,
        http.client.HTTPException,
        TimeoutError,
        ValueError,
        zipfile.BadZipFile,
        zipfile.LargeZipFile,
    ) as error:
        log(f"[Update] Update check or installation failed: {error}")
        return False
    finally:
        if not handed_off and stage_root is not None and stage_root.exists():
            try:
                shutil.rmtree(stage_root)
            except OSError as error:
                log(f"[Update] Could not clean temporary update files: {error}")


def auto_update_if_available(launch_args, log):
    if sys.platform != "win32" or not getattr(sys, "frozen", False):
        return False
    install_root = Path(sys.executable).parent
    if not (install_root / "fastroads-update.json").is_file():
        log("[Update] This build has no release metadata; automatic updates are disabled")
        return False
    return update_and_relaunch(install_root, launch_args, log)
