# Releasing DataDoc

Publishing a GitHub release with a tag like `v1.0.0` runs
`.github/workflows/release.yml`. The tag sets the version and the run number
the build number. The workflow first tests the engine on disk images made with
each file system's own tools, then builds every package and attaches them to
the release. The workflow is written by the Giode CLI from `giode.toml`: after
changing `[ci]` or a platform section, run `giode ci -f` instead of editing it,
then redo the two edits noted above `[ci]` (the CLI pinned to
`@v0.1.35-alpha.1`, and `go-version-file: go.mod`).

| Where | What the release gets | Needs |
| --- | --- | --- |
| GitHub, Linux | `.deb` and `.rpm`, x64 | nothing |
| GitHub, Windows | `DataDoc-<version>-amd64-setup.exe` | nothing; a code signing certificate stops SmartScreen warnings |
| Microsoft Store | `DataDoc-<version>-amd64.msix`, uploaded in Partner Center | a Partner Center account, the reserved name, and the elevation check below |
| GitHub, macOS | a universal `.dmg`, signed and notarized | a Developer ID certificate, or macOS refuses to open it |
| Google Play | the `.aab`, uploaded in Play Console (the `.apk` beside it is for GitHub) | a Play developer account and the upload keystore |
| GitHub, iOS | `.ipa` for the devices in the profile | an iOS certificate and an ad hoc or development profile |

### Stores DataDoc is not submitted to

DataDoc's whole job is reading drives sector by sector. Two stores can't allow
that, so DataDoc isn't offered in them and `giode.toml` has no build for them.

- **Mac App Store.** Every Mac App Store app runs in the App Sandbox, and
  no entitlement lets a sandboxed app open `/dev/disk*` or `/dev/rdisk*`. The
  way DataDoc gets that access, relaunching as root through the administrator
  prompt, is also refused by the sandbox and by App Review (guideline
  2.4.5: no escalation to root). The sandbox blocks the rest of what the Mac
  app reads too: iPhones through `usbmuxd`'s socket in `/var/run`, Android
  phones through the user's `adb`, and the iPhone backups in
  `~/Library/Application Support/MobileSync`. What would be left, disk images
  and the four tools on folders the user picks, isn't a data recovery app,
  and selling it as one would mislead buyers. The notarized `.dmg` is the Mac
  release.
- **Apple App Store (iOS).** iOS gives no app raw access to the phone's
  storage or to a drive plugged into it, so DataDoc on an iPhone can't get
  back anything deleted from the iPhone, a USB drive or a card. It only scans
  disk images and backups already copied into Files, which have to be made on
  a computer where DataDoc recovers directly. A "data recovery" listing would
  promise what the app can't do (guidelines 2.3.1 and 4.2). The iOS build
  stays: the release carries an `.ipa` when the iOS secrets are set, and
  `giode.toml` keeps the iOS settings ready (no permissions, an empty privacy
  manifest, `ITSAppUsesNonExemptEncryption = false`) should a disk-image
  version be wanted later.

## Repository secrets

Set these in GitHub under Settings → Secrets and variables → Actions. Each
`*_BASE64` secret is the file encoded with `base64 -i file | pbcopy` on macOS,
or `base64 -w0 file` on Linux. Without a secret, that step is skipped and the
rest of the release still builds (unsigned, or the iOS simulator check only).

| Secret | What it is |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64`, `GIODE_KEYSTORE_PASS` | The upload keystore (`release.keystore`) and its password: sign the `.aab` and `.apk` |
| `MACOS_CERT_BASE64`, `MACOS_CERT_PASS` | The "Developer ID Application" certificate as a `.p12`, and its password: signs the `.dmg` |
| `GIODE_NOTARY_ID`, `GIODE_NOTARY_TEAM_ID`, `GIODE_NOTARY_PASS` | Your Apple ID, team ID and an app-specific password: notarize the `.dmg` |
| `IOS_CERT_BASE64`, `IOS_CERT_PASS` | An "Apple Distribution" (or development) certificate as a `.p12` |
| `IOS_PROFILE_BASE64`, `IOS_TEAM_ID` | An ad hoc or development profile for `io.github.mehran9675.datadoc`, and the team ID |
| `WINDOWS_CERT_BASE64`, `GIODE_WINDOWS_CERT_PASS` | An Authenticode `.pfx` and its password: signs the setup program (optional; the Store signs the MSIX itself) |

`giode.toml` sets `android.keystore = "release.keystore"`: the workflow writes
that file from `ANDROID_KEYSTORE_BASE64`, and `giode build -release` fails
without the setting. `*.keystore` is in `.gitignore`; never commit it.

## Google Play

1. Open a Google Play developer account.
2. Create the upload keystore once, and keep it and its password safe:
   ```bash
   GIODE_KEYSTORE_PASS='...' giode keystore
   ```
   Add `ANDROID_KEYSTORE_BASE64` (from `base64 -w0 release.keystore`) and
   `GIODE_KEYSTORE_PASS` as repository secrets.
3. In Play Console, create the app "DataDoc" (app, free). Turn on **Play App
   Signing**: Google keeps the app signing key, and `release.keystore` is
   only the upload key. If it's ever lost, Play Console can reset the upload
   key.
4. Publish a release on GitHub. Upload the `.aab` from it to the internal
   testing track first. The package name is `io.github.mehran9675.datadoc`
   (`app.id`) and can't change after the first upload.
5. **Target API level.** Google Play requires Android 16 (API 36) for new apps
   and updates since 31 Aug 2026, so `[android]` sets `target_sdk = 36` (the
   CLI's default is 35, which Play rejects). Raise it when Play's next
   deadline comes; Play Console warns ahead of it.
6. **Permissions declarations** (Policy → App content). Play reviews these by
   hand, and they decide whether DataDoc is approved:
   - **All files access** (`MANAGE_EXTERNAL_STORAGE`). Core use: file
     recovery. DataDoc searches the whole shared storage for the Android
     trash, thumbnails and the copies other apps hide behind `.nomedia`, and
     saves recovered files to `Download/DataDoc`; the Storage Access
     Framework can't list other apps' hidden folders. Record a short video:
     the first scan of "This phone" opening the setting, then deleted
     photos found. If Play refuses, the app still installs and works on USB
     drives over OTG, disk images and rooted phones, but loses its scan of
     the phone's own storage: say so in the listing if that happens.
   - **Foreground service** (`dataSync`). Scans and recoveries keep running,
     with a notification, when the user leaves the app; they're started by
     the user and stop when done. Same video.
7. **Data safety:** no data collected, no data shared. DataDoc reads
   messages, contacts, photos and files only on the device and never sends
   them anywhere; Play counts on-device processing as not collected. No
   account, so no deletion request URL is needed.
8. **Content rating:** a utility; answer no to violence, sexuality, gambling,
   user-generated content and user interaction. Expect "Everyone" / PEGI 3.
9. **Privacy policy:** https://xmhrnx.github.io/remanance/#privacy
10. Store listing: the copy below. Category: Tools.

Root: DataDoc uses `su` only when the phone is already rooted and the user
opens its partitions or apps' databases. Play allows that; it doesn't root
anything.

## Microsoft Store

1. In Partner Center (the account that publishes EasySend and AnyFile),
   reserve the name "DataDoc". If it's taken, reserve another and set
   `app.name` for the MSIX build only, as AnyFile's workflow does.
2. In Product identity, check the values against `[windows.msix]`:
   - **Package/Identity/Name** → `identity_name` (now `MehranZahir.DataDoc`;
     change it if Partner Center shows something else).
   - **Package/Identity/Publisher** → `publisher`
     (`CN=B40E8F57-A7E0-40BB-8E74-5D051D0B578B`).
   - **Package/Properties/PublisherDisplayName** → `publisher_display_name`
     (`Mehran Zahiroddini`).
3. **Check elevation before the first submission.** Raw drives need
   administrator rights, and DataDoc gets them with "Restart as
   administrator" (a UAC prompt, then the app relaunches elevated). An MSIX
   app may need the `allowElevation` restricted capability for that, which
   giode-cli v0.1.35 doesn't declare (its package has only `runFullTrust`).
   On a Windows PC, sideload the release's `.msix` (signed with a test
   certificate whose subject is the publisher above, or through Partner
   Center's package flight), open a USB drive, click "Restart as
   administrator" and start a scan.
   - If it scans: submit the MSIX.
   - If it doesn't: don't submit the MSIX, since everything but disk images,
     phones and the tools would fail. Either wait for giode-cli to support
     `allowElevation`, or submit DataDoc as an **EXE app** instead: Partner
     Center takes the setup program by its release URL
     (`https://github.com/xmhrnx/remanance/releases/download/v<version>/DataDoc-<version>-amd64-setup.exe`),
     silent install switch `/S`. That needs the setup program signed with a
     certificate from a CA Windows trusts (`WINDOWS_CERT_BASE64`).
4. Upload `DataDoc-<version>-amd64.msix` in a submission. The Store signs it.
   - **runFullTrust:** DataDoc is a desktop app that reads drives, connected
     phones and the user's folders.
   - **Privacy policy:** https://xmhrnx.github.io/remanance/#privacy
   - **Category:** Utilities & tools. **Age rating (IARC):** a utility, no
     user interaction or content: 3+.
5. Store listing: the copy below.

## macOS (GitHub)

1. In the Apple Developer Program, create a "Developer ID Application"
   certificate, export it as a `.p12`, and add `MACOS_CERT_BASE64` and
   `MACOS_CERT_PASS`.
2. Create an app-specific password for your Apple ID and add
   `GIODE_NOTARY_ID`, `GIODE_NOTARY_TEAM_ID` and `GIODE_NOTARY_PASS`.
3. Publish a release: the `.dmg` is signed, notarized and stapled. The app
   isn't sandboxed and asks for the administrator password when a drive is
   opened. macOS asks once for Desktop, Documents, Downloads and removable
   drives when a tool first reads them (the reasons are in `[macos.plist]`).

## iOS (GitHub only)

1. Register the App ID `io.github.mehran9675.datadoc` in Certificates,
   Identifiers & Profiles, and create an ad hoc profile with the devices.
2. Add `IOS_CERT_BASE64`, `IOS_CERT_PASS`, `IOS_PROFILE_BASE64` and
   `IOS_TEAM_ID`. The release then carries an `.ipa`; without them the
   workflow only checks that the simulator build compiles.

See "Stores DataDoc is not submitted to" for why there is no App Store
submission.

## Store listing

Accurate to the app as built; change it when the app changes.

**Name:** DataDoc

**Short description** (Play, ≤80 characters):
Recover deleted photos, files and messages from drives, cards and phones.

**Short description** (Microsoft Store):
Free, open-source data recovery: deleted files, formatted drives and lost partitions.

**Full description:**

> DataDoc gets back files that were deleted, recovers drives that were
> formatted, and finds partitions that were lost. It reads the drive
> directly, with its own parsers for each file system, and never writes to it.
>
> - Deleted files with their names, folders and dates from NTFS, FAT, exFAT
>   and ext2/3/4.
> - Lost and deleted partitions, found by a deep scan of the whole drive.
> - Formatted drives: photos, videos, audio, documents and archives found by
>   their contents, including camera raw files.
> - Failing drives: copy the drive to an image once, then recover from the
>   image.
> - Deleted messages and records: deleted rows in the databases apps keep,
>   saved as spreadsheets.
> - A health estimate for every file, and previews before you recover.
>
> Four tools for the drive you still have: Disk space (a treemap of what
> takes room), Clean up (caches, temporary files and old downloads),
> Duplicates (exact copies and similar photos) and Rename (many files from
> a pattern, with undo).
>
> Nothing leaves your device: no account, no ads, no tracking, no internet
> connection. DataDoc is free and open source under the Apache License 2.0.

Platform paragraph to append:

- **Windows:** "Reads internal drives, USB drives and memory cards, and
  iPhones and Android phones connected by USB (iPhones need Apple Devices
  or iTunes; Android phones need USB debugging and adb). Asks for
  administrator rights when you open a drive."
- **Play:** "On the phone: everything in the Android trash, thumbnails and
  the copies apps hide, and USB drives and memory cards on an OTG adapter.
  With root, the phone's partitions and the deleted texts and chats in
  apps' databases. Phones encrypt each file and throw away a deleted
  file's key, so files deleted from the phone's own storage come back only
  where a copy or thumbnail survives."

**Category:** Tools (Play); Utilities & tools (Microsoft Store).

**Keywords** (for an Apple listing, should one be made; ≤100 characters):
`undelete,recover,deleted,photos,files,partition,sd card,usb,disk,restore`

**Screenshots:** none are in the repository yet. Take them from a scan of a
test image (`testdata/`), never of anyone's real files:
- Desktop (Microsoft Store, 1366×768 or larger): the drive list; results of
  a deep scan with health bars; a photo preview; the recover dialog; Disk
  space's treemap; Duplicates.
- Phone (Play, at least 2, portrait): the drive list with "This phone" and
  a USB drive; results; Clean up; Duplicates.

**App privacy / Data safety:** Data Not Collected. The privacy manifest
(`[privacy]`) declares no tracking and no collected data, and the disk space
API for Clean up's drive meter.

**Export compliance:** DataDoc uses only standard algorithms: AES and PBKDF2
to open the user's own encrypted iPhone backups, TLS to talk to a connected
iPhone, and SHA-256 to compare files. Nothing proprietary, and nothing that
protects communication with a server. `ITSAppUsesNonExemptEncryption = false`
is set for Apple; on Microsoft's and Google's forms, answer that the app uses
encryption only through standard algorithms, which is exempt.

**Age rating:** suitable for everyone. It shows the user's own files and
messages; it has no web access, chat or purchases.

## Linux

The `.deb` and `.rpm` are attached to the release. They list the system
libraries they need, so `apt` and `dnf` install those too. The app relaunches
through `pkexec` to read raw drives.
