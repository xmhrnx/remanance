package io.github.mehran9675.datadoc;

import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbDeviceConnection;
import android.hardware.usb.UsbEndpoint;
import android.hardware.usb.UsbInterface;
import android.hardware.usb.UsbManager;
import android.os.Build;

import java.util.HashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

// UsbBridge finds USB drives through Android's USB host API and opens
// them for the Go driver: Android grants the app the device's usbdevfs
// descriptor once the user allows it, with no root. Everything after
// that, the SCSI commands, is Go.
public final class UsbBridge {
	private static final String ACTION = "io.github.mehran9675.datadoc.USB_PERMISSION";
	private static final int CLASS_MASS_STORAGE = 8;
	private static final int SUBCLASS_SCSI = 6;
	private static final int PROTOCOL_BULK_ONLY = 0x50;
	private static final int XFER_BULK = 2;
	private static final int DIR_IN = 0x80;

	private static final HashMap<String, UsbDeviceConnection> open = new HashMap<>();

	private UsbBridge() {}

	private static UsbManager manager(Context ctx) {
		return (UsbManager) ctx.getSystemService(Context.USB_SERVICE);
	}

	private static UsbInterface massStorage(UsbDevice dev) {
		for (int i = 0; i < dev.getInterfaceCount(); i++) {
			UsbInterface in = dev.getInterface(i);
			if (in.getInterfaceClass() == CLASS_MASS_STORAGE
					&& in.getInterfaceSubclass() == SUBCLASS_SCSI
					&& in.getInterfaceProtocol() == PROTOCOL_BULK_ONLY) {
				return in;
			}
		}
		return null;
	}

	private static String clean(String s) {
		return s == null ? "" : s.replace('\t', ' ').replace('\n', ' ').trim();
	}

	// list returns one line per USB drive: device name, vendor and
	// product IDs, manufacturer, product, and 1 when the app may already
	// use it, separated by tabs.
	public static String list(Context ctx) {
		UsbManager m = manager(ctx);
		if (m == null) {
			return "";
		}
		StringBuilder out = new StringBuilder();
		for (UsbDevice dev : m.getDeviceList().values()) {
			if (massStorage(dev) == null) {
				continue;
			}
			String maker = "", product = "";
			if (Build.VERSION.SDK_INT >= 21) {
				maker = clean(dev.getManufacturerName());
				product = clean(dev.getProductName());
			}
			out.append(dev.getDeviceName()).append('\t')
				.append(dev.getVendorId()).append('\t')
				.append(dev.getProductId()).append('\t')
				.append(maker).append('\t')
				.append(product).append('\t')
				.append(m.hasPermission(dev) ? 1 : 0).append('\n');
		}
		return out.toString();
	}

	// open asks the user for the drive if needed, waiting for the answer,
	// claims its storage interface and returns "fd iface in out", or
	// "error: why". It must not run on the main thread.
	public static String open(Context ctx, String name) {
		UsbManager m = manager(ctx);
		UsbDevice dev = m == null ? null : m.getDeviceList().get(name);
		if (dev == null) {
			return "error: the drive was unplugged";
		}
		if (!m.hasPermission(dev) && !ask(ctx, m, dev)) {
			return "error: access to the drive wasn't allowed";
		}
		UsbInterface in = massStorage(dev);
		UsbEndpoint epIn = null, epOut = null;
		for (int i = 0; i < in.getEndpointCount(); i++) {
			UsbEndpoint ep = in.getEndpoint(i);
			if (ep.getType() != XFER_BULK) {
				continue;
			}
			if (ep.getDirection() == DIR_IN) {
				epIn = ep;
			} else {
				epOut = ep;
			}
		}
		if (epIn == null || epOut == null) {
			return "error: the drive has no bulk endpoints";
		}
		UsbDeviceConnection conn = m.openDevice(dev);
		if (conn == null) {
			return "error: the drive couldn't be opened";
		}
		if (!conn.claimInterface(in, true)) {
			conn.close();
			return "error: another app is using the drive";
		}
		synchronized (open) {
			UsbDeviceConnection old = open.put(name, conn);
			if (old != null) {
				old.close();
			}
		}
		return conn.getFileDescriptor() + " " + in.getId() + " " + epIn.getAddress() + " " + epOut.getAddress();
	}

	public static void close(String name) {
		UsbDeviceConnection conn;
		synchronized (open) {
			conn = open.remove(name);
		}
		if (conn != null) {
			conn.close();
		}
	}

	private static boolean ask(Context ctx, UsbManager m, UsbDevice dev) {
		final CountDownLatch done = new CountDownLatch(1);
		final boolean[] granted = new boolean[1];
		BroadcastReceiver r = new BroadcastReceiver() {
			@Override
			public void onReceive(Context c, Intent intent) {
				granted[0] = intent.getBooleanExtra(UsbManager.EXTRA_PERMISSION_GRANTED, false);
				done.countDown();
			}
		};
		IntentFilter filter = new IntentFilter(ACTION);
		if (Build.VERSION.SDK_INT >= 33) {
			ctx.registerReceiver(r, filter, Context.RECEIVER_NOT_EXPORTED);
		} else {
			ctx.registerReceiver(r, filter);
		}
		try {
			Intent intent = new Intent(ACTION);
			intent.setPackage(ctx.getPackageName());
			// The system fills in the extras, so the intent has to be
			// mutable from Android 12 on.
			int flags = Build.VERSION.SDK_INT >= 31 ? 0x02000000 /* FLAG_MUTABLE */ : 0;
			m.requestPermission(dev, PendingIntent.getBroadcast(ctx, 0, intent, flags));
			done.await(2, TimeUnit.MINUTES);
		} catch (InterruptedException e) {
			return false;
		} finally {
			ctx.unregisterReceiver(r);
		}
		return granted[0];
	}
}
