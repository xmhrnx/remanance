#!/bin/sh
# Builds the disk images the engine tests recover from, with real file
# system tools, and a manifest of every file's SHA-256 for checking what
# comes back. Run from the repository root:
#
#   docker run --rm --privileged -v "$PWD/testdata:/t" debian:bookworm sh /t/mkimages.sh
#
# Images land in testdata/fixtures/ (git-ignored).
set -eu

apt-get update -qq >/dev/null
apt-get install -y -qq ntfs-3g dosfstools exfatprogs e2fsprogs python3 python3-pil attr >/dev/null

# Images on the bind mount are slow and lock up loop devices: build inside the
# container, copy out at the end.
OUT=/work
mkdir -p /t/fixtures
mkdir -p "$OUT"
MNT=/mnt/fx
mkdir -p "$MNT"
# Usually nothing is mounted by the end; umount failing then must not
# become the script's exit status.
trap 'umount "$MNT" 2>/dev/null || true' EXIT

# populate DIR writes the same tree of files into every file system.
populate() {
	python3 /t/populate.py "$1"
}

manifest() { # manifest DIR NAME: hashes before deleting
	python3 - "$1" "$2" <<'EOF'
import hashlib, json, os, sys
root, name = sys.argv[1], sys.argv[2]
out = []
for d, _, files in os.walk(root):
    for f in files:
        p = os.path.join(d, f)
        if "System Volume Information" in p or "lost+found" in p:
            continue
        with open(p, "rb") as fh:
            data = fh.read()
        out.append({"path": "/" + os.path.relpath(p, root).replace(os.sep, "/"), "size": len(data), "sha256": hashlib.sha256(data).hexdigest()})
json.dump(sorted(out, key=lambda x: x["path"]), open(f"/work/{name}.json", "w"), indent=1)
EOF
}

delete_some() {
	rm -f "$1/docs/report.txt" "$1/photos/beach.jpg" "$1/big.bin" "$1/tiny.txt" "$1/fragmented.bin"
	rm -rf "$1/projects"
	sync -f "$1"
}

# mountimg IMG mounts an image file. NTFS goes through ntfs-3g on the file
# itself: ntfs-3g on a loop device hangs under Docker Desktop.
mountimg() {
	if [ "$(blkid -o value -s TYPE "$1")" = ntfs ]; then
		ntfs-3g "$1" "$MNT"
	else
		case "$(blkid -o value -s TYPE "$1")" in
		vfat) mount -o loop,utf8 "$1" "$MNT" ;;
		*) mount -o loop "$1" "$MNT" ;;
		esac
	fi
}

build() { # build NAME MKFS
	name=$1; mkfs=$2
	img="$OUT/$name.img"
	rm -f "$img"
	truncate -s 96M "$img"
	sh -c "$mkfs $img" >/dev/null
	mountimg "$img"
	populate "$MNT"
	sync -f "$MNT"
	manifest "$MNT" "$name"
	delete_some "$MNT"
	umount "$MNT"
	echo "built $name"
}

build ntfs "mkntfs -F -Q -L Fixture"
build fat32 "mkfs.vfat -F 32 -n FIXTURE"
build fat16 "mkfs.vfat -F 16 -n FIXTURE16"
build exfat "mkfs.exfat -L Fixture"
build ext4 "mkfs.ext4 -q -F -L fixture"
build ext3 "mkfs.ext3 -q -F -L fixture3"

# A drive whose partition table is gone: an NTFS and a FAT32 partition
# with nothing describing them. The partitions are built as files and
# copied in.
img="$OUT/lostparts.img"
rm -f "$img"
truncate -s 200M "$img"
# No table is written at all: it gets wiped anyway, and partitioning
# tools call sync(), which a hung mount in Docker's VM blocks forever.
p2start=$((2048 + 90 * 2048))
part="$OUT/part.img"
rm -f "$part"; truncate -s 90M "$part"
mkntfs -F -Q -L LostNTFS "$part" >/dev/null
mountimg "$part"; populate "$MNT"; sync -f "$MNT"; manifest "$MNT" lostparts-p1; umount "$MNT"
dd if="$part" of="$img" bs=512 seek=2048 conv=notrunc 2>/dev/null
rm -f "$part"; truncate -s 90M "$part"
mkfs.vfat -F 32 -n LOSTFAT "$part" >/dev/null
mountimg "$part"; populate "$MNT"; sync -f "$MNT"; manifest "$MNT" lostparts-p2; umount "$MNT"
dd if="$part" of="$img" bs=512 seek="$p2start" conv=notrunc 2>/dev/null
rm -f "$part"
dd if=/dev/zero of="$img" bs=512 count=34 conv=notrunc 2>/dev/null
size=$(stat -c %s "$img")
dd if=/dev/zero of="$img" bs=512 seek=$((size / 512 - 33)) count=33 conv=notrunc 2>/dev/null
echo "built lostparts"

# A FAT32 volume quick-formatted over its files: only carving finds them.
img="$OUT/reformatted.img"
rm -f "$img"
truncate -s 96M "$img"
mkfs.vfat -F 32 "$img" >/dev/null
mountimg "$img"; populate "$MNT"; sync -f "$MNT"; manifest "$MNT" reformatted; umount "$MNT"
mkfs.vfat -F 32 -n NEW "$img" >/dev/null
echo "built reformatted"

cp "$OUT"/* /t/fixtures/
chmod -R a+rw /t/fixtures
