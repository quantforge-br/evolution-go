#!/usr/bin/env python3
"""Exercise the integration probe against local HTTP fixtures only."""

from contextlib import contextmanager, redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import json
import os
from pathlib import Path
import sys
import threading
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import probe

PRIVATE_STATUS = {"status": "active", "mode": "local", "telemetry_enabled": False}


@contextmanager
def fixture_server(overrides=None):
    requests = []
    responses = {
        "/server/ok": (200, {"status": "ok"}, {}),
        "/license/status": (200, PRIVATE_STATUS, {}),
        "/instance/status": (
            200, {"data": {"Connected": True, "LoggedIn": True, "Name": "PRIVATE_NAME"}}, {}
        ),
        "/instance/all": (
            200, {"data": [{"id": "PRIVATE_ID", "token": "ROW_SECRET"}]}, {}
        ),
    }
    responses.update(overrides or {})

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers.get("apikey")))
            code, payload, headers = responses.get(self.path, (404, {}, {}))
            body = json.dumps(payload).encode() if not isinstance(payload, bytes) else payload
            self.send_response(code)
            for name, value in headers.items():
                self.send_header(name, value)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            try:
                self.wfile.write(body)
            except (BrokenPipeError, ConnectionResetError):
                pass

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port), requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


class ProbeTests(unittest.TestCase):
    def test_public_probe_does_not_send_credentials(self):
        with fixture_server() as (base, requests):
            report = probe.run_probe(base, "public", "UNUSED_SECRET", 2)
        self.assertTrue(report["ok"])
        self.assertEqual(requests, [("GET", "/server/ok", None), ("GET", "/license/status", None)])

    def test_instance_scope_uses_only_instance_token_and_redacts(self):
        with fixture_server() as (base, requests):
            report = probe.run_probe(base, "instance", "INSTANCE_SECRET", 2)
        self.assertTrue(report["ok"])
        self.assertTrue(report["checks"][-1]["session_ready"])
        self.assertEqual(requests[-1], ("GET", "/instance/status", "INSTANCE_SECRET"))
        self.assertEqual([request[2] for request in requests[:-1]], [None, None])
        self.assertNotIn("INSTANCE_SECRET", json.dumps(report))
        self.assertNotIn("PRIVATE_NAME", json.dumps(report))

    def test_admin_scope_does_not_expose_instance_rows(self):
        with fixture_server() as (base, requests):
            report = probe.run_probe(base, "admin", "ADMIN_SECRET", 2)
        self.assertTrue(report["ok"])
        self.assertEqual(requests[-1], ("GET", "/instance/all", "ADMIN_SECRET"))
        for value in ("ADMIN_SECRET", "ROW_SECRET", "PRIVATE_ID"):
            self.assertNotIn(value, json.dumps(report))

    def test_disconnected_session_is_not_reported_ready(self):
        responses = {"/instance/status": (200, {"data": {"Connected": False, "LoggedIn": False}}, {})}
        with fixture_server(responses) as (base, _):
            report = probe.run_probe(base, "instance", "TOKEN", 2)
        self.assertTrue(report["ok"])
        self.assertFalse(report["checks"][-1]["session_ready"])

    def test_lowercase_status_fields_are_not_accepted(self):
        responses = {"/instance/status": (200, {"data": {"connected": True, "loggedIn": True}}, {})}
        with fixture_server(responses) as (base, _):
            report = probe.run_probe(base, "instance", "TOKEN", 2)
        self.assertFalse(report["ok"])
        self.assertEqual(report["checks"][-1]["error"], "unexpected_status_shape")

    def test_wrong_private_mode_or_telemetry_fails(self):
        for status in ({**PRIVATE_STATUS, "mode": "remote"}, {**PRIVATE_STATUS, "telemetry_enabled": True}):
            with self.subTest(status=status), fixture_server({"/license/status": (200, status, {})}) as (base, _):
                self.assertFalse(probe.run_probe(base, "public", None, 2)["ok"])

    def test_error_response_does_not_echo_body(self):
        with fixture_server({"/instance/status": (401, {"error": "BODY_SECRET"}, {})}) as (base, _):
            report = probe.run_probe(base, "instance", "TOKEN", 2)
        self.assertFalse(report["ok"])
        self.assertEqual(report["checks"][-1]["http_status"], 401)
        self.assertNotIn("BODY_SECRET", json.dumps(report))

    def test_cross_origin_redirect_does_not_forward_credentials(self):
        with fixture_server() as (target, target_requests):
            redirect = (302, b"", {"Location": target + "/instance/status"})
            with fixture_server({"/instance/status": redirect}) as (base, _):
                report = probe.run_probe(base, "instance", "SECRET", 2)
        self.assertFalse(report["ok"])
        self.assertEqual(report["checks"][-1]["error"], "redirect_rejected")
        self.assertEqual(target_requests, [])

    def test_same_origin_redirect_is_also_rejected(self):
        redirect = (307, b"", {"Location": "/never-follow"})
        with fixture_server({"/instance/status": redirect}) as (base, requests):
            report = probe.run_probe(base, "instance", "SECRET", 2)
        self.assertFalse(report["ok"])
        self.assertNotIn("/never-follow", [request[1] for request in requests])

    def test_invalid_json_and_large_bodies_fail_without_echoing(self):
        for body, error in ((b"BODY_SECRET", "invalid_json"), (b"x" * 1025, "response_too_large")):
            with self.subTest(error=error), patch.object(probe, "MAX_RESPONSE_BYTES", 1024):
                with fixture_server({"/server/ok": (200, body, {})}) as (base, _):
                    report = probe.run_probe(base, "public", None, 2)
            self.assertFalse(report["ok"])
            self.assertEqual(report["checks"][0]["error"], error)
            self.assertNotIn("BODY_SECRET", json.dumps(report))

    def test_connection_errors_are_sanitized(self):
        with patch.object(probe.urllib.request.OpenerDirector, "open", side_effect=TimeoutError("SECRET")):
            report = probe.run_probe("https://example.invalid", "instance", "TOKEN", 2)
        self.assertFalse(report["ok"])
        self.assertEqual(report["checks"][0]["error"], "connection_failed")
        self.assertNotIn("SECRET", json.dumps(report))

    def test_proxy_environment_is_ignored(self):
        with fixture_server() as (base, _):
            with patch.dict(os.environ, {"http_proxy": "http://127.0.0.1:1", "no_proxy": ""}):
                self.assertTrue(probe.run_probe(base, "public", None, 2)["ok"])

    def test_base_url_validation(self):
        for url in (
            "https://user:SECRET@example.com", "https://example.com/api", "https://example.com?q=SECRET",
            "https://example.com#SECRET", "https://example.com:99999", "file:///tmp/x",
            "http://private-service:8080", "https://example.com\n",
        ):
            with self.subTest(url=url), self.assertRaises(ValueError):
                probe.validate_base_url(url)
        self.assertEqual(probe.validate_base_url("https://example.com/"), "https://example.com")
        self.assertEqual(probe.validate_base_url("http://127.0.0.1:8080"), "http://127.0.0.1:8080")
        self.assertEqual(probe.validate_base_url("http://service:8080", True), "http://service:8080")

    def test_missing_or_invalid_credentials_stop_before_network(self):
        for value in ("", "SECRET\nINJECTED", "Sëcret"):
            with self.subTest(value=value), patch.dict(os.environ, {"EVOLUTION_GO_INSTANCE_TOKEN": value}):
                with patch.object(probe, "run_probe") as run, redirect_stdout(io.StringIO()) as output:
                    code = probe.main(["--scope", "instance"])
                self.assertEqual(code, 2)
                self.assertFalse(run.called)
                self.assertNotIn("SECRET", output.getvalue())
                self.assertEqual(json.loads(output.getvalue())["error"], "invalid_configuration")

    def test_invalid_timeout_and_url_are_not_echoed(self):
        for args in (["--timeout", "nan"], ["--timeout", "0"], ["--base-url", "https://u:SECRET@host"]):
            with self.subTest(args=args), redirect_stdout(io.StringIO()) as output:
                code = probe.main(args)
            self.assertEqual(code, 2)
            self.assertNotIn("SECRET", output.getvalue())

    def test_cli_exit_status_and_json(self):
        with fixture_server() as (base, _), redirect_stdout(io.StringIO()) as output:
            self.assertEqual(probe.main(["--base-url", base]), 0)
        self.assertTrue(json.loads(output.getvalue())["ok"])
        with fixture_server({"/server/ok": (500, {}, {})}) as (base, _):
            with redirect_stdout(io.StringIO()) as output:
                self.assertEqual(probe.main(["--base-url", base]), 1)
        self.assertFalse(json.loads(output.getvalue())["ok"])


if __name__ == "__main__":
    unittest.main()
