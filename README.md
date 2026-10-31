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
- **Deleted messages and records**: apps keep texts, call logs,
  contacts, notes and chats in SQLite databases, which leave a deleted
  row in place until they need the space. Remanence finds those rows in
  every database on a phone or backup (and in the known ones on any
  drive), in free pages, in the free space inside pages and in the
  write-ahead log, and saves them as spreadsheets (CSV): messages with
  their dates and contacts for iPhone and Android, WhatsApp, Notes,
  Safari, calls and contacts, and any other table as it is.
- **Phones**: see [Phones](#phones) below.

Every file gets a health estimate: Remanence checks whether its
clusters were reused since it was deleted. You can also preview photos,
text and raw bytes before you recover anything.

## Platforms

| | Drives | Notes |
|---|---|---|
| **Windows** | Internal, USB, SD cards, iPhones and Android phones by USB | Asks for administrator rights when you open a drive. |
| **macOS** | Internal, USB, SD cards, iPhones and Android phones by USB | Asks for your password to read raw disks. |
| **Linux** | Internal, USB, SD cards, iPhones and Android phones by USB | Relaunches through `pkexec` for raw access. |
| **Android** | The phone's own storage, USB drives and SD card readers on an OTG adapter, and with root the phone's partitions, memory card and apps' databases | See [Phones](#phones). Recovered files go to `Download/Remanence`. |
| **iOS / iPadOS** | Disk images | iOS gives no app raw access to storage. Connect the iPhone to a computer and recover from there instead. Recovered files appear in the Files app. |

## Phones

A phone encrypts its storage file by file and throws away a deleted
file's key, so its sectors can't give the file back, with or without
root. What Remanence can still find:

- **The Android trash.** Since Android 11, many apps delete photos and
  videos by moving them to a system trash for 30 days. Remanence lists
  everything in it, from every app.
- **Copies apps hide.** Thumbnails, and media that messaging and other
  apps keep in hidden folders or behind a `.nomedia` file, often
  outlive the original. A deep scan also searches caches, such as the
  gallery's thumbnail databases, for the pictures inside them.

  Reading other apps' files needs Android's "All files access": the
  first scan of "This phone" opens the setting.
- **USB drives and memory cards** on an OTG adapter, **no root
  needed**. Remanence uses Android's USB host API and its own USB mass
  storage driver (SCSI over Bulk-Only Transport). Android asks once
  per drive.
- **Deleted texts and chats on rooted phones.** With root, "This
  phone's apps" lists the databases apps keep in `/data/data`: texts and
  picture messages, the call log, contacts, WhatsApp and the rest. A
  scan searches each for deleted rows (see [What it
  recovers](#what-it-recovers)).
- **Rooted phones' storage**: the phone's userdata partition and a
  memory card in the phone's slot, read raw through `su`. A card set up
  as portable storage isn't encrypted, so a deep scan finds its deleted
  files like any card's; userdata gives back files only on older phones
  that didn't encrypt it. Recovered files are saved to the phone's own
  storage, so recover the files that matter most first.
- **Android phones, from a computer.** Connect the phone by USB with
  USB debugging turned on (in Developer options), and Remanence on
  Windows, macOS and Linux lists it through ADB, Android's debug bridge:
  its storage, searched as above, and on a rooted phone its apps'
  databases. It needs Android's platform tools (`adb`) installed; it
  starts the adb server itself.
- **iPhones, from a computer.** iOS lets no app read its storage, so an
  iPhone is recovered from a backup:
  - Connect the iPhone by USB and choose "Back up" in Remanence. It
    backs the phone up itself, the way Finder does, after the phone asks
    you to trust the computer. On Windows this needs Apple Devices or
    iTunes installed, for Apple's USB driver; on Linux, `usbmuxd`.
  - Or scan a backup Finder or iTunes already made: Remanence lists
    them, and opens any backup folder you pick. On macOS, give Remanence
    Full Disk Access to see them.

  A backup holds the phone's photos, those in Recently Deleted
  included, and every app's files and databases, where deleted
  messages, calls, notes and chats are found. **Encrypted backups**
  open with their password, the one set for encrypted backups in Finder
  or iTunes (or on the phone), and hold more: call history, Health data
  and saved passwords among them. Photos kept only in iCloud aren't in a
  backup.

## Install

Download the installer for your system from
[Releases](https://github.com/xmhrnx/remanance/releases):

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

```bash
remanence-cli backup
```

`list` also shows the iPhone backups on the computer and the phones
connected by USB. `backup` backs up the connected iPhone; `scan` and
`recover` take a backup's folder as the source, and read an encrypted
one with the password in `REMANENCE_BACKUP_PASSWORD`.

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
- Reading an iPhone straight over USB without a backup, or from iCloud.
- Deleted rows a database has already overwritten, or apps that wipe
  deleted rows on purpose (SQLite's `secure_delete`).

## License

Remanence is licensed under the Apache License, version 2.0 (see
[LICENSE](LICENSE)). The bundled fonts, Inter and JetBrains Mono, are
under the SIL Open Font License; their licenses are in
`ui/theme/fonts/`.
