<p align="center"><img src="assets/icon.png" width="112" alt="Remanence"></p>

<h1 align="center">Remanence</h1>

<p align="center">Free, open-source data recovery for Windows, macOS, Linux and Android.</p>

Remanence gets back files that were deleted, recovers drives that were
formatted, and finds partitions that were lost. It reads the drive
directly, through its own parsers for each file system, and never
writes to it.

## What it recovers

- **Deleted files** with their names, folders and dates, from the file
  system's own records:
  - **NTFS**: MFT records, including fragmented files from their
    data runs.
  - **FAT12/16/32**: long file names. FAT clears a deleted file's
    cluster chain, so the data is read contiguously from its first
    cluster, which is how most files were written.
  - **exFAT**: directory entries the system marked unused.
  - **ext2/3/4**: inodes plus the names the journal still holds.
    Since Linux 5.13 the kernel wipes a deleted file's directory
    entry, so the journal is often the only place its name survives.
- **Lost and deleted partitions**: a deep scan looks for file system
  boot sectors and superblocks across the drive, with MBR and GPT
  tables read on the way.
- **Formatted drives and raw data**: carving finds files by their
  contents where no file system record is left. It recognizes:
  - Photos: JPEG, PNG, GIF, BMP, TIFF, HEIC, AVIF, WebP, PSD, and camera
    raw from Canon, Nikon, Sony, Fujifilm, Olympus, Panasonic and
    Pentax, plus DNG.
  - Video: MP4, MOV, 3GP, AVI, MKV, WebM, WMV.
  - Audio: MP3, WAV, AIFF, Ogg.
  - Documents: PDF, RTF, Office (old and new), OpenDocument, EPUB.
  - Archives and other files: ZIP, 7z, RAR, gzip, SQLite, Windows
    programs, Android apps.
- **Failing drives**: copy a drive to an image once, then recover from
  the image as often as you like.

Every file gets a health estimate: Remanence checks whether its
clusters were reused since it was deleted. You can also preview photos,
text and raw bytes before you recover anything.

## Platforms

| | Drives | Notes |
|---|---|---|
| **Windows** | Internal, USB, SD cards | Asks for administrator rights when you open a drive. |
| **macOS** | Internal, USB, SD cards | Asks for your password to read raw disks. |
| **Linux** | Internal, USB, SD cards | Relaunches through `pkexec` for raw access. |
| **Android** | USB drives and SD card readers on an OTG adapter | **No root needed.** It uses Android's USB host API and its own USB mass storage driver (SCSI over Bulk-Only Transport). Android asks once per drive. Recovered files go to `Download/Remanence`. |
| **iOS / iPadOS** | Disk images | iOS gives no app raw access to storage, so Remanence on iOS scans disk images. Recovered files appear in the Files app. |

The phone's own internal storage is encrypted per file and can't be
read raw without root on Android, or at all on iOS. To recover photos
deleted from a phone, look first in its "Recently deleted" album, then
in your backups.

## Install

Download the installer for your system from
[Releases](https://github.com/mehran9675/remanence/releases):

- Windows: `.exe` installer or `.msix`
- macOS: `.dmg`
- Linux: `.deb` or `.rpm`
- Android: `.apk`
- iOS: `.ipa`, on releases built with a provisioning profile

**Don't install Remanence on the drive you're recovering from.** Every
file written there can overwrite what you want back.

## Command line

`remanence-cli` does everything the app does, for scripts and servers:

```bash
remanence-cli list
```

```bash
remanence-cli scan -deep /dev/sdb
```

```bash
remanence-cli recover -to ~/Recovered -type jpg,mp4 /dev/sdb
```

```bash
remanence-cli image /dev/sdb sdb.img
```

## Building

Remanence is written in Go. The UI uses
[giode](https://github.com/mehran9675/giode), and installers are built
with [giode-cli](https://github.com/mehran9675/giode-cli).

```bash
go run ./cmd/remanence
```

```bash
giode build windows-installer dmg deb android
```

The engine (`engine/`) is pure Go with no dependencies beyond the
standard library and `x/sys`. To test it against real file systems,
build the disk images first. This needs Docker, because it formats
images with each file system's own tools:

```bash
docker run --rm --privileged -v "$PWD/testdata:/t" debian:bookworm sh /t/mkimages.sh
```

```bash
go test ./engine/...
```

Releases are built by GitHub Actions when a release is published. The
workflow is written by `giode ci -f` from the `[ci]` section of
`giode.toml`, so change it there rather than editing
`.github/workflows/release.yml`. iOS builds are signed when the
repository has the `IOS_CERT_BASE64`, `IOS_CERT_PASS` and
`IOS_PROFILE_BASE64` secrets.

`giode build android` compiles the Android USB bridge
(`ui/platform/androidusb/UsbBridge.java`) into the APK.

## Not yet

- APFS, HFS+, Btrfs, XFS and ReFS. Deleted files on these drives are
  found only by carving, without their names.
- RAID reconstruction and recovery over the network.
- Reading iPhone backups.

## License

Remanence is licensed under the GPL, version 3 or later (see
[LICENSE](LICENSE)). The bundled fonts, Inter and JetBrains Mono, are
under the SIL Open Font License; their licenses are in
`ui/theme/fonts/`.
