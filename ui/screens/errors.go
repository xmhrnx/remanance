package screens

import "errors"

// errNoPath is shown when a picked image can't be read in place, as on
// phones, where the picker hands out a stream.
var errNoPath = errors.New("that file can't be read in place; copy the image to this device's storage and open it from there")
