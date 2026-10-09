"""The whole upload against a running Couchverse, opt-in: a draft movie with a two-language
file, its audio tag and a subtitle, the server's probed inventory of it, then all of it removed.

    COUCHPUSH_LIVE_SERVER=http://localhost:8080 COUCHPUSH_LIVE_USER=admin \
    COUCHPUSH_LIVE_PASSWORD=... python -m unittest tests.test_live

It needs ffmpeg and an admin account, and signs in as a device it signs out again at the end.
"""

import os
import shutil
import subprocess
import tempfile
import time
import unittest
from pathlib import Path

from couchpush.api.client import CouchverseClient, OffsetConflict
from couchpush.core.uploader import upload_file
from couchpush.media.plan import normalize_lang

SERVER = os.environ.get("COUCHPUSH_LIVE_SERVER", "")


@unittest.skipUnless(SERVER and shutil.which("ffmpeg"), "set COUCHPUSH_LIVE_SERVER to a Couchverse to upload to")
class LiveServerTests(unittest.TestCase):
    def setUp(self):
        self.client = CouchverseClient(SERVER)
        user = self.client.login(os.environ.get("COUCHPUSH_LIVE_USER", "admin"),
                                 os.environ.get("COUCHPUSH_LIVE_PASSWORD", "admin"))
        self.addCleanup(self.client.session.close)
        self.addCleanup(self.client.logout)
        self.assertEqual(user["role"], "admin")
        self.assertEqual(self.client.me()["username"], user["username"])

    def test_a_two_language_movie_uploads_and_is_removed_again(self):
        client = self.client
        title_id = client.create_movie("CouchPush Live Check", 2026)
        self.addCleanup(lambda: client.session.delete(f"{client.api}/admin/titles/{title_id}",
                                                      timeout=client.timeout))
        with tempfile.TemporaryDirectory() as directory:
            movie = str(Path(directory) / "CouchPush Live Check (2026).mp4")
            generated = subprocess.run(
                ["ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
                 "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=3",
                 "-f", "lavfi", "-i", "sine=frequency=440:duration=3",
                 "-map", "0:v", "-map", "1:a", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast",
                 "-c:a", "aac", "-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=ces", movie],
                capture_output=True, text=True, timeout=60)
            self.assertEqual(generated.returncode, 0, generated.stderr)
            # small chunks, so the upload takes several appends
            upload_id = upload_file(client, movie, chunk_size=64 * 1024)
            self.assertEqual(client.upload_status(upload_id)["receivedBytes"], os.path.getsize(movie))
            media_id = client.upload_complete(upload_id, "movies", title_id=title_id)
            client.set_media_audio(media_id, "en", "primary")
            subtitle = Path(directory) / "cs.vtt"
            subtitle.write_text("WEBVTT\n\n00:00:00.500 --> 00:00:02.000\nZkouška\n", "utf-8")
            self.assertEqual(client.upload_subtitle(media_id, "cs", str(subtitle))["lang"], "cs")

        # the probe fills in the file's tracks; until it has, they are unknown, not none
        deadline = time.monotonic() + 60
        while True:
            media = next(m for m in client.title_detail(title_id).media_files if m.id == media_id)
            if media.audio_streams is not None or time.monotonic() > deadline:
                break
            time.sleep(1)
        self.assertIsNotNone(media.audio_streams, "the server never probed the upload")
        self.assertEqual([normalize_lang(a.lang) for a in media.audio_streams], ["en", "cs"])
        self.assertEqual((media.audio_lang, media.audio_role), ("en", "primary"))

        # an append that is not where the server is answers where it is
        stray = client.upload_create("stray.mp4", 10)
        with self.assertRaises(OffsetConflict) as conflict:
            client.upload_append(stray, 5, b"12345")
        self.assertEqual(conflict.exception.server_offset, 0)
        client.upload_abort(stray)

        client.delete_media_file(media_id)
        self.assertEqual([m.id for m in client.title_detail(title_id).media_files], [])


if __name__ == "__main__":
    unittest.main()
