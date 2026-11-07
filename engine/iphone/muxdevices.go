package iphone

import (
	"net"

	"github.com/xmhrnx/remanance/engine/plist"
)

// devices lists the phones connected by USB.
func devices() ([]device, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	defer m.Close()
	d, err := m.request("ListDevices", nil)
	if err != nil {
		return nil, err
	}
	var out []device
	list, _ := d["DeviceList"].([]any)
	for _, x := range list {
		p := plist.Dict(plist.Dict(x)["Properties"])
		if p["ConnectionType"] != "USB" {
			continue // over Wi-Fi: too slow for a backup to be worth it
		}
		out = append(out, device{id: plist.Int(p["DeviceID"]), udid: plist.String(p["SerialNumber"])})
	}
	return out, nil
}

// connect opens a connection to a port on the phone, through usbmuxd.
func connect(id int64, port uint16) (net.Conn, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	// The port goes in network byte order.
	_, err = m.request("Connect", map[string]any{"DeviceID": id, "PortNumber": int64(port>>8 | port<<8)})
	if err != nil {
		m.Close()
		return nil, err
	}
	return m.conn, nil
}

// pairRecord reads the record of this computer's pairing with the
// phone, kept by usbmuxd; nil if they haven't paired.
func pairRecord(udid string) (map[string]any, error) {
	m, err := openMux()
	if err != nil {
		return nil, err
	}
	defer m.Close()
	d, err := m.request("ReadPairRecord", map[string]any{"PairRecordID": udid})
	if err != nil {
		return nil, nil
	}
	data, _ := d["PairRecordData"].([]byte)
	v, err := plist.Parse(data)
	if err != nil {
		return nil, nil
	}
	return plist.Dict(v), nil
}

// savePairRecord gives usbmuxd a new pairing to keep.
func savePairRecord(dev device, record map[string]any) error {
	data, err := plist.MarshalXML(record)
	if err != nil {
		return err
	}
	m, err := openMux()
	if err != nil {
		return err
	}
	defer m.Close()
	_, err = m.request("SavePairRecord", map[string]any{"PairRecordID": dev.udid, "PairRecordData": data, "DeviceID": dev.id})
	return err
}

// systemBUID is the identifier usbmuxd gives this computer.
func systemBUID() (string, error) {
	m, err := openMux()
	if err != nil {
		return "", err
	}
	defer m.Close()
	d, err := m.request("ReadBUID", nil)
	if err != nil {
		return "", err
	}
	return plist.String(d["BUID"]), nil
}
