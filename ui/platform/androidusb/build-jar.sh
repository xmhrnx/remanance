#!/bin/sh
# Builds usbbridge_android.jar, which gogio packs into the APK. It
# compiles against the SDK's android.jar ($ANDROID_JAR, or android-34
# under $ANDROID_HOME) when there is one, otherwise against the stubs
# here, which declare only what UsbBridge uses.
set -e
cd "$(dirname "$0")"
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
sdk=${ANDROID_JAR:-${ANDROID_HOME:-/nonexistent}/platforms/android-34/android.jar}
if [ -f "$sdk" ]; then
	javac --release 8 -nowarn -classpath "$sdk" -d "$out/classes" UsbBridge.java
else
	javac --release 8 -nowarn -d "$out/stubs" $(find stubs -name '*.java')
	javac --release 8 -nowarn -classpath "$out/stubs" -d "$out/classes" UsbBridge.java
fi
jar cf usbbridge_android.jar -C "$out/classes" .
