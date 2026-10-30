package androidusb

import (
	"strings"

	"gioui.org/app"
	"git.wow.st/gmp/jni"

	"github.com/mehran9675/remanence/engine/disk"
	"github.com/mehran9675/remanence/engine/disk/usbmass"
)

const bridgeClass = "io/github/mehran9675/remanence/UsbBridge"

// Register adds the USB drives to disk.List.
func Register() { disk.Register(provider{}) }

type provider struct{}

// call runs a static method of UsbBridge taking the app context first
// and returning a String.
func call(method, sig string, args ...string) (string, error) {
	var out string
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		ctx := jni.Object(app.AppContext())
		cls, err := jni.LoadClass(env, jni.ClassLoaderFor(env, ctx), bridgeClass)
		if err != nil {
			return err
		}
		vals := []jni.Value{jni.Value(ctx)}
		for _, a := range args {
			vals = append(vals, jni.Value(jni.JavaString(env, a)))
		}
		obj, err := jni.CallStaticObjectMethod(env, cls, jni.GetStaticMethodID(env, cls, method, sig), vals...)
		if err != nil {
			return err
		}
		out = jni.GoString(env, jni.String(obj))
		return nil
	})
	return out, err
}

func (provider) List() ([]disk.Info, error) {
	list, err := call("list", "(Landroid/content/Context;)Ljava/lang/String;")
	if err != nil {
		return nil, err
	}
	return parse(list), nil
}

func (provider) Handles(id string) bool { return strings.HasPrefix(id, prefix) }

func (provider) Open(info disk.Info) (disk.Device, error) {
	name := strings.TrimPrefix(info.ID, prefix)
	answer, err := call("open", "(Landroid/content/Context;Ljava/lang/String;)Ljava/lang/String;", name)
	if err != nil {
		return nil, err
	}
	fd, iface, in, out, err := parseOpen(answer)
	if err != nil {
		return nil, err
	}
	c := closer(name)
	dev, err := usbmass.Open(usbmass.NewDevFS(fd, iface, in, out), info, c)
	if err != nil {
		c.Close()
		return nil, err
	}
	return dev, nil
}

// closer releases the drive: Android closes the descriptor with the
// connection.
type closer string

func (c closer) Close() error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		cls, err := jni.LoadClass(env, jni.ClassLoaderFor(env, jni.Object(app.AppContext())), bridgeClass)
		if err != nil {
			return err
		}
		return jni.CallStaticVoidMethod(env, cls, jni.GetStaticMethodID(env, cls, "close", "(Ljava/lang/String;)V"),
			jni.Value(jni.JavaString(env, string(c))))
	})
}
