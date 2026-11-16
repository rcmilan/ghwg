//go:build js

package game

import (
	"fmt"
	"syscall/js"
)

// saveKey is prefixed: itch's html.itch.zone origin, and its localStorage, are shared by every game.
const saveKey = "ghosthotel.save.v1"

func platformRead() (b []byte, err error) {
	defer jsRecover(&err)
	v := js.Global().Get("localStorage").Call("getItem", saveKey)
	if v.IsNull() || v.IsUndefined() {
		return nil, nil
	}
	return []byte(v.String()), nil
}

func platformWrite(b []byte) (err error) {
	defer jsRecover(&err)
	js.Global().Get("localStorage").Call("setItem", saveKey, string(b))
	return nil
}

// jsRecover: localStorage throws when the browser blocks it or it is full.
func jsRecover(err *error) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("localStorage: %v", r)
	}
}
