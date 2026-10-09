"""Device sign-in against the auth contract, and the saved token's keyring entry."""

import json
import sys
import threading
import types
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

from couchpush import config
from couchpush.api.client import AuthError, CouchverseClient, device_name

ADMIN = {"id": 1, "username": "admin", "displayName": "Admin", "role": "admin"}


class AuthHandler(BaseHTTPRequestHandler):
    """POST /auth/token, GET /auth/me and POST /auth/logout as the server answers them."""

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length") or 0))
        self.server.requests.append((self.command, self.path, self.headers.get("Authorization"),
                                     self.headers.get("Cookie"), json.loads(body or b"null")))
        if self.path == "/api/v1/auth/token":
            sign_in = json.loads(body)
            if sign_in["password"] != "secret":
                self.reply(401, {"error": {"code": "invalid_credentials", "message": "invalid username or password"}})
            elif not 1 <= len(sign_in["deviceName"]) <= 60 or sign_in["platform"] != "desktop":
                self.reply(400, {"error": {"code": "bad_request", "message": "invalid request body"}})
            else:
                self.reply(200, {"token": "tok", "deviceId": "00000000-0000-4000-8000-000000000001", "user": ADMIN})
        elif self.path == "/api/v1/auth/logout":
            self.reply(204, None)

    def do_GET(self):
        self.server.requests.append((self.command, self.path, self.headers.get("Authorization"),
                                     self.headers.get("Cookie"), None))
        if self.headers.get("Authorization") == "Bearer tok":
            self.reply(200, ADMIN)
        else:
            self.reply(401, {"error": {"code": "unauthorized", "message": "sign in"}})

    def reply(self, status, result):
        self.send_response(status)
        if result is None:
            self.end_headers()
            return
        encoded = json.dumps(result).encode()
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, *_args):
        pass


class DeviceSessionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), AuthHandler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(2)

    def setUp(self):
        self.server.requests = []
        self.client = CouchverseClient(f"http://127.0.0.1:{self.server.server_port}/")
        self.addCleanup(self.client.session.close)

    def test_sign_in_is_a_named_desktop_device_and_requests_carry_its_token(self):
        self.assertEqual(self.client.login("admin", "secret"), ADMIN)
        _, path, _, _, body = self.server.requests[0]
        self.assertEqual(path, "/api/v1/auth/token")
        self.assertEqual(body, {"username": "admin", "password": "secret",
                                "deviceName": device_name(), "platform": "desktop"})
        self.assertTrue(device_name().startswith("CouchPush on "))
        self.assertEqual(self.client.token, "tok")
        self.assertEqual(self.client.me(), ADMIN)
        _, path, auth, cookie, _ = self.server.requests[-1]
        self.assertEqual((path, auth, cookie), ("/api/v1/auth/me", "Bearer tok", None))

    def test_a_wrong_password_is_an_auth_error(self):
        with self.assertRaises(AuthError) as raised:
            self.client.login("admin", "wrong")
        self.assertEqual(raised.exception.status, 401)
        self.assertEqual(self.client.token, "")

    def test_a_saved_token_signs_in_and_signing_out_ends_its_session(self):
        self.client.restore_session("tok")
        self.assertEqual(self.client.me()["username"], "admin")
        self.client.logout()
        self.assertEqual(self.server.requests[-1][:3], ("POST", "/api/v1/auth/logout", "Bearer tok"))
        self.assertEqual(self.client.token, "")
        with self.assertRaises(AuthError):
            self.client.me()
        count = len(self.server.requests)
        self.client.logout()
        self.assertEqual(len(self.server.requests), count, "a signed-out client has nothing to end")

    def test_the_device_name_fits_the_servers_limit(self):
        with patch("couchpush.api.client.socket.gethostname", return_value="a" * 80 + ".lan"):
            self.assertEqual(len(device_name()), 60)
        with patch("couchpush.api.client.socket.gethostname", return_value="Studio-PC.local"):
            self.assertEqual(device_name(), "CouchPush on Studio-PC")


class SavedTokenTests(unittest.TestCase):
    def setUp(self):
        store = {}
        keyring = types.SimpleNamespace(
            set_password=lambda service, user, value: store.__setitem__((service, user), value),
            get_password=lambda service, user: store.get((service, user)),
            delete_password=lambda service, user: store.pop((service, user)),
        )
        self.store = store
        patcher = patch.dict(sys.modules, {"keyring": keyring})
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_a_token_is_only_ever_restored_for_its_own_server(self):
        config.save_session_token("https://tv.example.com", "tok")
        self.assertEqual(config.load_session_token("https://tv.example.com"), "tok")
        self.assertEqual(config.load_session_token("http://192.168.0.69:8080"), "")
        config.clear_session_token()
        self.assertEqual(config.load_session_token("https://tv.example.com"), "")

    def test_an_older_builds_session_cookie_is_not_a_token(self):
        self.store[("couchpush", "session")] = json.dumps({"value": "cookie", "expiresAt": None})
        self.assertEqual(config.load_session_token("https://tv.example.com"), "")
        self.store[("couchpush", "session")] = "not json"
        self.assertEqual(config.load_session_token("https://tv.example.com"), "")


if __name__ == "__main__":
    unittest.main()
