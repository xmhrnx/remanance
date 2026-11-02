// Package androidstore offers the phone's own shared storage for a scan
// on Android: its trash, the copies apps hide from the gallery, and
// caches with pictures inside (see engine/phonestore). Reading other
// apps' files takes "All files access", which StorageBridge.java asks
// for; giode build compiles it into the APK (android.java in
// giode.toml).
package androidstore

import "errors"

// bridgeClass is StorageBridge.java's class, as JNI names it.
const bridgeClass = "io/github/mehran9675/datadoc/StorageBridge"

// ErrAccess means the user hasn't allowed DataDoc to read every app's
// files yet; the settings to allow it have just opened.
var ErrAccess = errors.New("DataDoc needs \"All files access\" to look through this phone's storage. " +
	"Turn it on in the settings that just opened, then come back and start the scan again")

// ErrFileAccess is ErrAccess for the file tools.
var ErrFileAccess = errors.New("DataDoc needs \"All files access\" to read the folders on this phone. " +
	"Turn it on in the settings that just opened, then come back and try again")
