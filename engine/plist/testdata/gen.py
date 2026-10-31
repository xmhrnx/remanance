# Writes the property lists plist_test.go reads, with Python's plistlib:
#   python testdata/gen.py
import datetime, os, plistlib

here = os.path.dirname(os.path.abspath(__file__))
value = {
    "Name": "Sam's iPhone",
    "Unicode": "Fotos – été",
    "Count": 42,
    "Big": 2**40,
    "Negative": -7,
    "Ratio": 0.5,
    "Yes": True,
    "No": False,
    "When": datetime.datetime(2026, 10, 30, 12, 0, 0),
    "Blob": bytes(range(40)),
    "List": ["a", 1, [2, 3]],
    "Long": "x" * 300,
}
with open(os.path.join(here, "xml.plist"), "wb") as f:
    plistlib.dump(value, f, fmt=plistlib.FMT_XML)
value["Ref"] = plistlib.UID(5)
with open(os.path.join(here, "binary.plist"), "wb") as f:
    plistlib.dump(value, f, fmt=plistlib.FMT_BINARY)
