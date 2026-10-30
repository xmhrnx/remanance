package android.hardware.usb;
public class UsbDeviceConnection {
	public boolean claimInterface(UsbInterface i, boolean force) { return false; }
	public int getFileDescriptor() { return -1; }
	public void close() {}
}
