# Writes test.db for sqlite_test.go with Python's sqlite3:
#   python testdata/gen.py
# Small pages make a b-tree several levels deep from a few hundred rows.
import os, sqlite3

here = os.path.dirname(os.path.abspath(__file__))
path = os.path.join(here, "test.db")
if os.path.exists(path):
    os.remove(path)
db = sqlite3.connect(path)
db.execute("PRAGMA page_size = 1024")
db.execute("CREATE TABLE Files (fileID TEXT PRIMARY KEY, domain TEXT, relativePath TEXT, flags INTEGER, file BLOB)")
db.execute('CREATE TABLE "notes" (id INTEGER PRIMARY KEY, title TEXT, score REAL, big BLOB)')
for i in range(600):
    db.execute("INSERT INTO Files VALUES (?, ?, ?, ?, ?)",
               ("%040x" % i, "CameraRollDomain", "Media/DCIM/100APPLE/IMG_%04d.HEIC" % i, 1, bytes([i % 256]) * (i % 50)))
db.execute("INSERT INTO notes (title, score, big) VALUES (?, ?, ?)", ("Café ☕", -1.5, bytes(range(256)) * 40))
db.execute("INSERT INTO notes (title, score, big) VALUES (?, ?, ?)", ("small", 2**40, None))
db.execute("ALTER TABLE notes ADD COLUMN extra TEXT")
db.commit()
db.close()
