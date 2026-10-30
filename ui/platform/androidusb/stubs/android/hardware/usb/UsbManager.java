package android.hardware.usb;
public class UsbManager {
	public static final String EXTRA_PERMISSION_GRANTED = "permission";
	public java.util.HashMap<String, UsbDevice> getDeviceList() { return null; }
	public boolean hasPermission(UsbDevice d) { return false; }
	public void requestPermission(UsbDevice d, android.app.PendingIntent pi) {}
	public UsbDeviceConnection openDevice(UsbDevice d) { return null; }
}
