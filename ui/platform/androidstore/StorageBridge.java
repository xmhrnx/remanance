package io.github.mehran9675.remanence;

import android.Manifest;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.Settings;

// StorageBridge asks for the access the Go side needs to look through
// the phone's shared storage: "All files access" since Android 11, the
// storage permission before. Android grants either only in its
// settings, so request opens them on the app's page.
public final class StorageBridge {
	private StorageBridge() {}

	// access answers "yes" when the app may read every app's files.
	public static String access(Context ctx) {
		if (Build.VERSION.SDK_INT >= 30) {
			return Environment.isExternalStorageManager() ? "yes" : "no";
		}
		if (Build.VERSION.SDK_INT >= 23) {
			return ctx.checkSelfPermission(Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED ? "yes" : "no";
		}
		return "yes"; // granted at install
	}

	// request opens the settings page where the user grants it.
	public static void request(Context ctx) {
		Uri app = Uri.parse("package:" + ctx.getPackageName());
		Intent i = Build.VERSION.SDK_INT >= 30
				? new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, app)
				: new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, app);
		i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
		try {
			ctx.startActivity(i);
		} catch (Exception e) {
			// Some phones leave out the per-app page; the list of apps works.
			if (Build.VERSION.SDK_INT >= 30) {
				Intent all = new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION);
				all.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
				ctx.startActivity(all);
			}
		}
	}
}
