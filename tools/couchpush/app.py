"""PyInstaller entry point (PyInstaller bundles a script, not `python -m couchpush`)."""

import sys

from couchpush.__main__ import main

if __name__ == "__main__":
    sys.exit(main())
