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

# deleted.db: messages, some deleted one by one (left in their pages'
# free space) and a run of them together (whole pages freed).
path = os.path.join(here, "deleted.db")
if os.path.exists(path):
    os.remove(path)
db = sqlite3.connect(path)
db.execute("PRAGMA secure_delete = OFF")
db.execute("CREATE TABLE handle (ROWID INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL, service TEXT)")
db.execute("CREATE TABLE message (ROWID INTEGER PRIMARY KEY AUTOINCREMENT, guid TEXT UNIQUE NOT NULL, text TEXT, handle_id INTEGER DEFAULT 0, date INTEGER, is_from_me INTEGER DEFAULT 0)")
db.execute("INSERT INTO handle (id, service) VALUES ('+15550100', 'SMS'), ('sam@example.com', 'iMessage')")
for i in range(1, 301):
    db.execute("INSERT INTO message (guid, text, handle_id, date, is_from_me) VALUES (?, ?, ?, ?, ?)",
               ("G-%05d" % i, "Message number %d: see you at %d o'clock" % (i, i % 12 + 1), i % 2 + 1, 700000000000000000 + i * 1000000000, i % 3 == 0))
db.commit()
db.execute("DELETE FROM message WHERE ROWID % 10 = 3 AND ROWID < 200")
db.execute("DELETE FROM message WHERE ROWID BETWEEN 200 AND 260")
db.commit()
db.close()

# wal.db and wal.db-wal: the same, with the deletions still in the
# write-ahead log, copied while the database is open.
import shutil
for name in ("wal.db", "wal.db-wal"):
    if os.path.exists(os.path.join(here, name)):
        os.remove(os.path.join(here, name))
tmp = os.path.join(here, "tmp-wal.db")
for name in (tmp, tmp + "-wal", tmp + "-shm"):
    if os.path.exists(name):
        os.remove(name)
db = sqlite3.connect(tmp)
db.execute("PRAGMA secure_delete = OFF")
db.execute("PRAGMA journal_mode = WAL")
db.execute("PRAGMA wal_autocheckpoint = 0")
db.execute("CREATE TABLE sms (_id INTEGER PRIMARY KEY, address TEXT, date INTEGER, body TEXT, type INTEGER)")
for i in range(1, 101):
    db.execute("INSERT INTO sms (address, date, body, type) VALUES (?, ?, ?, ?)",
               ("+1555010%d" % (i % 10), 1700000000000 + i * 60000, "Text %d about the trip to the lake" % i, 1 + i % 2))
db.commit()
db.execute("PRAGMA wal_checkpoint(TRUNCATE)")
db.execute("DELETE FROM sms WHERE _id % 7 = 0")
db.execute("UPDATE sms SET type = 5 WHERE _id = 1")
db.commit()
shutil.copy(tmp, os.path.join(here, "wal.db"))
shutil.copy(tmp + "-wal", os.path.join(here, "wal.db-wal"))
db.close()
for name in (tmp, tmp + "-wal", tmp + "-shm"):
    if os.path.exists(name):
        os.remove(name)
