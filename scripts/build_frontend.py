"""Build the frontend static tree consumed by the Go server."""

import json
import os
import re
import shutil
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
FRONTEND = ROOT / "frontend"
DIST = ROOT / "dist"
STATIC_SUFFIXES = {
    ".css",
    ".html",
    ".ico",
    ".jpg",
    ".jpeg",
    ".js",
    ".json",
    ".png",
    ".svg",
    ".webmanifest",
    ".webp",
}


def copy_static_files() -> None:
    if DIST.exists():
        shutil.rmtree(DIST)
    DIST.mkdir()

    for path in FRONTEND.iterdir():
        if path.is_file() and path.suffix.lower() in STATIC_SUFFIXES:
            shutil.copy2(path, DIST / path.name)
        elif path.is_dir():
            for source in path.rglob("*"):
                if source.is_file() and source.suffix.lower() in STATIC_SUFFIXES:
                    destination = DIST / source.relative_to(FRONTEND)
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copy2(source, destination)


def configure_api_url() -> None:
    api_url = os.environ.get("WEDDINGHUB_API_URL", "").strip()
    if not api_url:
        # A same-origin Go deployment needs no injected URL: api-client.js uses
        # location.origin when the page is served over HTTP(S).
        return

    (DIST / "api-config.js").write_text(
        "window.WEDDINGHUB_API_URL = " + json.dumps(api_url) + ";\n",
        encoding="utf-8",
    )

    api_client_tag = re.compile(r'(<script\s+src=["\'])([^"\']*api-client\.js)(["\'])')
    for page in DIST.rglob("*.html"):
        content = page.read_text(encoding="utf-8")

        def add_config(match: re.Match[str]) -> str:
            config_path = Path(os.path.relpath(DIST / "api-config.js", page.parent)).as_posix()
            return f'<script src="{config_path}"></script>' + match.group(0)

        content = api_client_tag.sub(add_config, content)
        page.write_text(content, encoding="utf-8")


if __name__ == "__main__":
    copy_static_files()
    configure_api_url()
