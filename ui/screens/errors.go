package screens

import "github.com/xmhrnx/remanance/locales"

import "errors"

// errNoPath is shown when a picked image can't be read in place, as on
// phones, where the picker hands out a stream.
var errNoPath = errors.New(locales.T("errors.that_file_can_t_be_read"))
