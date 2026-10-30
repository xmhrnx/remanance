package android.hardware.usb;
public class UsbInterface {
	public int getId() { return 0; }
	public int getInterfaceClass() { return 0; }
	public int getInterfaceSubclass() { return 0; }
	public int getInterfaceProtocol() { return 0; }
	public int getEndpointCount() { return 0; }
	public UsbEndpoint getEndpoint(int i) { return null; }
}
