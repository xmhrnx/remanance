"""Writes the fixture file tree into a mounted file system.

Content is generated from fixed seeds, so every image holds the same
files and tests can compare recovered bytes with the manifest.
"""
import io
import os
import random
import sqlite3
import sys
import zipfile

from PIL import Image, ImageDraw

root = sys.argv[1]
rnd = random.Random(1234)


def write(path, data):
    full = os.path.join(root, path)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    with open(full, "wb") as f:
        f.write(data)


def noise(n):
    return rnd.randbytes(n)


def picture(w, h, seed, fmt, **kw):
    r = random.Random(seed)
    img = Image.new("RGB", (w, h), (r.randrange(256), r.randrange(256), r.randrange(256)))
    d = ImageDraw.Draw(img)
    for _ in range(40):
        x, y = r.randrange(w), r.randrange(h)
        d.ellipse((x, y, x + r.randrange(10, 200), y + r.randrange(10, 200)),
                  fill=(r.randrange(256), r.randrange(256), r.randrange(256)))
    b = io.BytesIO()
    img.save(b, fmt, **kw)
    return b.getvalue()


write("tiny.txt", b"Small enough to live inside the MFT record.\n")
write("docs/report.txt", b"".join(b"Quarterly report line %d\n" % i for i in range(3000)))
write("docs/notes.md", b"# Notes\n\n" + b"Some notes.\n" * 500)
write("docs/Ünïcödé naïve résumé.txt", "Ünïcödé content ✓\n".encode() * 200)
write("docs/a very long file name that goes well past the old eight dot three limit.txt", b"long name\n" * 100)
write("photos/beach.jpg", picture(1600, 1200, 1, "JPEG", quality=90))
write("photos/mountain.jpg", picture(1200, 900, 2, "JPEG", quality=85))
write("photos/diagram.png", picture(800, 600, 3, "PNG"))
write("photos/old.gif", picture(320, 240, 4, "GIF"))
write("photos/scan.bmp", picture(300, 200, 5, "BMP"))
write("big.bin", noise(3 * 1024 * 1024 + 123))

z = io.BytesIO()
with zipfile.ZipFile(z, "w", zipfile.ZIP_DEFLATED) as zf:
    zf.writestr("readme.txt", "inside a zip\n" * 300)
    zf.writestr("data.bin", noise(20000))
write("projects/archive.zip", z.getvalue())
write("projects/src/main.go", b"package main\n\nfunc main() {}\n" * 50)
write("projects/src/deep/deeper/deepest.txt", b"deep\n" * 1000)

db_path = os.path.join(root, "projects", "contacts.db")
os.makedirs(os.path.dirname(db_path), exist_ok=True)
con = sqlite3.connect(db_path)
con.execute("create table people(id integer primary key, name text, phone text)")
con.executemany("insert into people(name, phone) values(?, ?)", [("person %d" % i, "555-%04d" % i) for i in range(2000)])
con.commit()
con.close()

# A fragmented file: grow two files in turns so their clusters interleave.
fa = open(os.path.join(root, "fragmented.bin"), "wb")
fb = open(os.path.join(root, "filler.bin"), "wb")
for i in range(40):
    fa.write(noise(16384))
    fa.flush()
    os.fsync(fa.fileno())
    fb.write(noise(16384))
    fb.flush()
    os.fsync(fb.fileno())
fa.close()
fb.close()
