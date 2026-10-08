"""Exercise serialized requests against the admin title creation contract."""

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import requests

from couchpush.api.client import CouchverseClient, CouchverseError


class TitleHandler(BaseHTTPRequestHandler):
    # catalog.TitleInput is decoded with DisallowUnknownFields; status is
    # controlled by the server, not a field accepted by POST /admin/titles.
    fields = {"kind", "name", "overview", "year", "contentRating", "runtimeMinutes",
              "genres", "metadataLanguages"}

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.server.bodies.append(body)
        if set(body) - self.fields:
            status, result = 400, {"error": {"code": "bad_request", "message": "invalid request body"}}
        elif not body.get("name") or body.get("kind") != "movie":
            status, result = 400, {"error": {"code": "bad_request", "message": "name and kind (movie|series) are required"}}
        else:
            status, result = 201, {"id": "potter-title", "status": "draft"}
        encoded = json.dumps(result).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def log_message(self, *_args):
        pass


class ClientTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = ThreadingHTTPServer(("127.0.0.1", 0), TitleHandler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(2)

    def setUp(self):
        self.server.bodies = []
        self.client = CouchverseClient(f"http://127.0.0.1:{self.server.server_port}")
        self.addCleanup(self.client.session.close)

    def test_new_movie_creation_accepts_czech_name_and_server_controlled_draft_status(self):
        name = "Harry Potter 1 a Kámen mudrců"
        self.assertEqual(self.client.create_movie(name, 2001), "potter-title")
        self.assertEqual(self.server.bodies, [{"kind": "movie", "name": name, "year": 2001}])

    def test_new_movie_without_year_is_accepted(self):
        self.assertEqual(self.client.create_movie("Pelíšky"), "potter-title")
        self.assertIsNone(self.server.bodies[0]["year"])

    def test_rejected_request_identifies_the_endpoint(self):
        with self.assertRaises(CouchverseError) as raised:
            self.client.create_movie("", 2001)
        self.assertEqual(raised.exception.request, "POST /api/v1/admin/titles")
        self.assertIn("name and kind", str(raised.exception))

    def test_error_context_excludes_credentials_and_query_values(self):
        response = requests.Response()
        response.status_code = 400
        response._content = b'{"error":{"code":"bad_request","message":"invalid request body"}}'
        response.request = requests.Request(
            "POST", "https://admin:password@example.invalid/api/v1/admin/titles?secret=token"
        ).prepare()
        with self.assertRaises(CouchverseError) as raised:
            self.client._check(response)
        self.assertIn("POST /api/v1/admin/titles", str(raised.exception))
        self.assertNotIn("password", str(raised.exception))
        self.assertNotIn("token", str(raised.exception))


if __name__ == "__main__":
    unittest.main()
