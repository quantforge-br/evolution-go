#!/usr/bin/env python3
"""Probe a private Evolution Go installation without changing account state."""

import argparse
import ipaddress
import json
import math
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_BASE_URL = "https://evolutiongo.diogosrodrigues.com.br"
MAX_RESPONSE_BYTES = 1024 * 1024


class NoRedirect(urllib.request.HTTPRedirectHandler):
    """Refuse redirects, including same-host redirects, before another request."""

    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


def validate_base_url(value, allow_http=False):
    """Require a base origin without embedded credentials, queries or fragments."""
    if any(ord(char) <= 32 or ord(char) == 127 for char in value):
        raise ValueError("Invalid base URL")
    parsed = urllib.parse.urlsplit(value)
    if (
        parsed.scheme not in ("https", "http")
        or not parsed.hostname
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
        or "?" in value
        or "#" in value
        or parsed.path not in ("", "/")
    ):
        raise ValueError("Use an origin URL without credentials or an API prefix")
    # Accessing port validates malformed/out-of-range values.
    _ = parsed.port
    try:
        loopback = ipaddress.ip_address(parsed.hostname).is_loopback
    except ValueError:
        loopback = parsed.hostname.lower() == "localhost"
    if parsed.scheme == "http" and not (loopback or allow_http):
        raise ValueError("HTTP requires loopback or explicit --allow-http")
    return value.rstrip("/")


def get_json(opener, url, token, timeout):
    """Return a bounded JSON response without exposing bodies or exception text."""
    headers = {
        "Accept": "application/json",
        "User-Agent": "EvolutionGoIntegrationProbe/1.0",
    }
    if token:
        headers["apikey"] = token
    request = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with opener.open(request, timeout=timeout) as response:
            status = response.status
            body = response.read(MAX_RESPONSE_BYTES + 1)
    except urllib.error.HTTPError as error:
        status = error.code
        error.close()
        return status, None, "redirect_rejected" if 300 <= status < 400 else "http_error"
    except (urllib.error.URLError, TimeoutError, OSError, ValueError):
        return None, None, "connection_failed"
    if len(body) > MAX_RESPONSE_BYTES:
        return status, None, "response_too_large"
    try:
        payload = json.loads(body)
    except (ValueError, UnicodeError, RecursionError):
        return status, None, "invalid_json"
    return status, payload, None


def run_probe(base_url, scope, token, timeout):
    # Do not silently forward credentials through an environment HTTP proxy.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    checks = []
    for name, path, credential in [
        ("health", "/server/ok", None),
        ("private_runtime", "/license/status", None),
        *([("instance_auth", "/instance/status", token)] if scope == "instance" else []),
        *([("admin_auth", "/instance/all", token)] if scope == "admin" else []),
    ]:
        status, payload, error = get_json(opener, base_url + path, credential, timeout)
        check = {"name": name, "path": path, "http_status": status, "ok": False}
        if error:
            check["error"] = error
        elif status != 200 or not isinstance(payload, dict):
            check["error"] = "unexpected_response"
        elif name == "health":
            check["ok"] = payload.get("status") == "ok"
        elif name == "private_runtime":
            check["ok"] = (
                payload.get("mode") == "local"
                and payload.get("telemetry_enabled") is False
                and payload.get("status") == "active"
            )
        elif name == "instance_auth":
            data = payload.get("data")
            if isinstance(data, dict) and all(
                isinstance(data.get(key), bool) for key in ("Connected", "LoggedIn")
            ):
                check["ok"] = True
                check["connected"] = data["Connected"]
                check["logged_in"] = data["LoggedIn"]
                check["session_ready"] = data["Connected"] and data["LoggedIn"]
            else:
                check["error"] = "unexpected_status_shape"
        elif name == "admin_auth":
            # Do not return row counts, names, IDs, QR codes or instance tokens.
            check["ok"] = isinstance(payload.get("data"), list)
        checks.append(check)
    return {
        "ok": all(check["ok"] for check in checks),
        "scope": scope,
        "checks": checks,
        "note": "Read-only HTTP checks; no pairing, send, delivery or callback test performed.",
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default=os.getenv("EVOLUTION_GO_BASE_URL", DEFAULT_BASE_URL))
    parser.add_argument("--scope", choices=("public", "instance", "admin"), default="public")
    parser.add_argument("--timeout", type=float, default=15.0)
    parser.add_argument(
        "--allow-http", action="store_true",
        help="Explicitly allow plaintext HTTP to a verified private non-loopback origin",
    )
    args = parser.parse_args(argv)
    try:
        if not math.isfinite(args.timeout) or not 0 < args.timeout <= 60:
            raise ValueError("Timeout must be positive and at most 60 seconds")
        base_url = validate_base_url(args.base_url, args.allow_http)
        variable = {
            "instance": "EVOLUTION_GO_INSTANCE_TOKEN",
            "admin": "EVOLUTION_GO_ADMIN_KEY",
        }.get(args.scope)
        token = os.getenv(variable, "").strip() if variable else None
        if variable and not token:
            raise ValueError("Missing " + variable)
        if token and (any(ord(char) < 33 or ord(char) > 126 for char in token)):
            raise ValueError("Credential must use printable ASCII without whitespace")
    except ValueError:
        # No supplied values are echoed: even a malformed URL might contain a key.
        print(json.dumps({
            "ok": False,
            "error": "invalid_configuration",
            "hint": "Use an HTTPS origin (or approved HTTP), a timeout in (0, 60], and the scoped credential environment variable.",
        }))
        return 2
    report = run_probe(base_url, args.scope, token, args.timeout)
    print(json.dumps(report, indent=2))
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
