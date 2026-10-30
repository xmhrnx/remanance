# Writes a small iPhone backup in Finder's layout for backup_test.go:
#   python testdata/gen.py
# Info.plist, Manifest.plist, Manifest.db listing the files, the files
# under their hashed names, and a Photos library with one photo in
# Recently Deleted.
import datetime, hashlib, os, plistlib, shutil, sqlite3

here = os.path.dirname(os.path.abspath(__file__))
root = os.path.join(here, "backup", "00008110-000A1B2C3D4E5F60")
shutil.rmtree(os.path.join(here, "backup"), ignore_errors=True)
os.makedirs(root)

with open(os.path.join(root, "Info.plist"), "wb") as f:
    plistlib.dump({"Device Name": "Test iPhone", "Product Name": "iPhone 15", "Product Version": "18.1",
                   "Last Backup Date": datetime.datetime(2026, 10, 1, 9, 30)}, f, fmt=plistlib.FMT_XML)
with open(os.path.join(root, "Manifest.plist"), "wb") as f:
    plistlib.dump({"IsEncrypted": False, "Version": "10.0"}, f, fmt=plistlib.FMT_BINARY)

photos = os.path.join(here, "photos.sqlite")
if os.path.exists(photos):
    os.remove(photos)
p = sqlite3.connect(photos)
p.execute("CREATE TABLE ZASSET (Z_PK INTEGER PRIMARY KEY, ZDIRECTORY VARCHAR, ZFILENAME VARCHAR, ZTRASHEDSTATE INTEGER, ZTRASHEDDATE TIMESTAMP)")
p.execute("INSERT INTO ZASSET VALUES (1, 'DCIM/100APPLE', 'IMG_0001.HEIC', 0, NULL)")
p.execute("INSERT INTO ZASSET VALUES (2, 'DCIM/100APPLE', 'IMG_0002.HEIC', 1, 815000000.0)")
p.commit()
p.close()
with open(photos, "rb") as f:
    photos_data = f.read()
os.remove(photos)

def mbfile(size, path):
    return plistlib.dumps({
        "$version": 100000, "$archiver": "NSKeyedArchiver", "$top": {"root": plistlib.UID(1)},
        "$objects": ["$null",
                     {"$class": plistlib.UID(3), "Size": size, "LastModified": 1790000000, "Birth": 1780000000,
                      "Mode": 0o100644, "RelativePath": plistlib.UID(2), "Flags": 0},
                     path,
                     {"$classname": "MBFile", "$classes": ["MBFile", "NSObject"]}]}, fmt=plistlib.FMT_BINARY)

files = [
    ("CameraRollDomain", "Media/DCIM/100APPLE/IMG_0001.HEIC", b"photo one"),
    ("CameraRollDomain", "Media/DCIM/100APPLE/IMG_0002.HEIC", b"photo two, deleted"),
    ("CameraRollDomain", "Media/DCIM/100APPLE/IMG_0002.MOV", b"its live video"),
    ("CameraRollDomain", "Media/PhotoData/Photos.sqlite", photos_data),
    ("AppDomain-net.whatsapp.WhatsApp", "Documents/note.txt", b"hello"),
    ("HomeDomain", "Library/SMS/sms.db", b"not really a database"),
]
m = sqlite3.connect(os.path.join(root, "Manifest.db"))
m.execute("CREATE TABLE Files (fileID TEXT PRIMARY KEY, domain TEXT, relativePath TEXT, flags INTEGER, file BLOB)")
m.execute("INSERT INTO Files VALUES (?, ?, ?, 2, NULL)", (hashlib.sha1(b"CameraRollDomain-Media").hexdigest(), "CameraRollDomain", "Media"))
for domain, rel, data in files:
    fid = hashlib.sha1((domain + "-" + rel).encode()).hexdigest()
    os.makedirs(os.path.join(root, fid[:2]), exist_ok=True)
    with open(os.path.join(root, fid[:2], fid), "wb") as f:
        f.write(data)
    m.execute("INSERT INTO Files VALUES (?, ?, ?, 1, ?)", (fid, domain, rel, mbfile(len(data), rel)))
# Listed, but its file is missing from the folder.
m.execute("INSERT INTO Files VALUES (?, ?, ?, 1, NULL)", ("ff" + "0" * 38, "HomeDomain", "Library/gone.txt"))
m.commit()
m.close()
