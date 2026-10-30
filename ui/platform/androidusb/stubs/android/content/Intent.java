package android.content;
public class Intent {
	public Intent(String action) {}
	public Intent setPackage(String p) { return this; }
	public boolean getBooleanExtra(String name, boolean def) { return def; }
}
