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

# The same files in an encrypted backup, password "test password", made
# as Finder makes them but with fewer key derivation rounds, so tests
# are quick: a keybag of class keys wrapped with the password's key,
# each file encrypted with its own key, wrapped with its class's.
import struct
from Crypto.Cipher import AES
from Crypto.Protocol.KDF import PBKDF2
from Crypto.Hash import SHA1, SHA256

def wrap(kek, key):
    # AES key wrap, RFC 3394.
    aes = AES.new(kek, AES.MODE_ECB)
    a = b"\xa6" * 8
    r = [key[i:i + 8] for i in range(0, len(key), 8)]
    n = len(r)
    for j in range(6):
        for i in range(n):
            b = aes.encrypt(a + r[i])
            t = n * j + i + 1
            a = (int.from_bytes(b[:8], "big") ^ t).to_bytes(8, "big")
            r[i] = b[8:]
    return a + b"".join(r)

def encrypt(key, data):
    pad = 16 - len(data) % 16
    return AES.new(key, AES.MODE_CBC, b"\0" * 16).encrypt(data + bytes([pad]) * pad)

def tlv(tag, value):
    if isinstance(value, int):
        value = struct.pack(">I", value)
    return tag.encode() + struct.pack(">I", len(value)) + value

def key_of(seed, n=32):
    return hashlib.sha256(seed.encode()).digest()[:n]

password = "test password"
dpsl, salt = key_of("dpsl", 20), key_of("salt", 20)
dpic, iters = 1000, 100
pkey = PBKDF2(PBKDF2(password, dpsl, 32, count=dpic, hmac_hash_module=SHA256), salt, 32, count=iters, hmac_hash_module=SHA1)
classes = {c: key_of("class %d" % c) for c in (1, 2, 3, 4)}
bag = tlv("VERS", 4) + tlv("TYPE", 1) + tlv("UUID", key_of("bag", 16)) + tlv("HMCK", b"\0" * 40) + tlv("WRAP", 0)
bag += tlv("SALT", salt) + tlv("ITER", iters) + tlv("DPWT", 1) + tlv("DPIC", dpic) + tlv("DPSL", dpsl)
for c, k in classes.items():
    bag += tlv("UUID", key_of("uuid %d" % c, 16)) + tlv("CLAS", c) + tlv("WRAP", 2) + tlv("KTYP", 0) + tlv("WPKY", wrap(pkey, k))
mkey = key_of("manifest")

eroot = os.path.join(here, "backup-encrypted", "00008110-000E2C3D4E5F6071")
shutil.rmtree(os.path.join(here, "backup-encrypted"), ignore_errors=True)
os.makedirs(eroot)
with open(os.path.join(eroot, "Info.plist"), "wb") as f:
    plistlib.dump({"Device Name": "Locked iPhone", "Product Name": "iPhone 16", "Product Version": "18.1",
                   "Last Backup Date": datetime.datetime(2026, 10, 2, 9, 30)}, f, fmt=plistlib.FMT_XML)
with open(os.path.join(eroot, "Manifest.plist"), "wb") as f:
    plistlib.dump({"IsEncrypted": True, "Version": "10.0", "BackupKeyBag": bag,
                   "ManifestKey": struct.pack("<I", 4) + wrap(classes[4], mkey)}, f, fmt=plistlib.FMT_BINARY)

def mbfile_encrypted(size, path, cls, wrapped):
    return plistlib.dumps({
        "$version": 100000, "$archiver": "NSKeyedArchiver", "$top": {"root": plistlib.UID(1)},
        "$objects": ["$null",
                     {"$class": plistlib.UID(5), "Size": size, "LastModified": 1790000000, "Birth": 1780000000,
                      "Mode": 0o100644, "RelativePath": plistlib.UID(2), "Flags": 0, "ProtectionClass": cls,
                      "EncryptionKey": plistlib.UID(3)},
                     path,
                     {"$class": plistlib.UID(4), "NS.data": struct.pack("<I", cls) + wrapped},
                     {"$classname": "NSMutableData", "$classes": ["NSMutableData", "NSData", "NSObject"]},
                     {"$classname": "MBFile", "$classes": ["MBFile", "NSObject"]}]}, fmt=plistlib.FMT_BINARY)

sms = os.path.join(here, "..", "..", "sqlite", "testdata", "deleted.db")
with open(sms, "rb") as f:
    sms_data = f.read()
efiles = [
    ("CameraRollDomain", "Media/DCIM/100APPLE/IMG_0001.HEIC", b"photo one, encrypted", 3),
    ("HomeDomain", "Library/SMS/sms.db", sms_data, 1),
    ("HomeDomain", "Library/CallHistoryDB/CallHistory.storedata", b"calls", 2),
]
mpath = os.path.join(here, "manifest.tmp")
if os.path.exists(mpath):
    os.remove(mpath)
m = sqlite3.connect(mpath)
m.execute("CREATE TABLE Files (fileID TEXT PRIMARY KEY, domain TEXT, relativePath TEXT, flags INTEGER, file BLOB)")
for domain, rel, data, cls in efiles:
    fid = hashlib.sha1((domain + "-" + rel).encode()).hexdigest()
    os.makedirs(os.path.join(eroot, fid[:2]), exist_ok=True)
    fkey = key_of("file " + rel)
    with open(os.path.join(eroot, fid[:2], fid), "wb") as f:
        f.write(encrypt(fkey, data))
    m.execute("INSERT INTO Files VALUES (?, ?, ?, 1, ?)", (fid, domain, rel, mbfile_encrypted(len(data), rel, cls, wrap(classes[cls], fkey))))
m.commit()
m.close()
with open(mpath, "rb") as f:
    mdata = f.read()
os.remove(mpath)
with open(os.path.join(eroot, "Manifest.db"), "wb") as f:
    f.write(encrypt(mkey, mdata))
