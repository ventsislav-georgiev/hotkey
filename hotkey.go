// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

// Package hotkey provides the basic facility to register a system-level
// global hotkey shortcut so that an application can be notified if a user
// triggers the desired hotkey. A hotkey must be a combination of modifiers
// and a single key.
//
// Note a platform specific detail on "macOS" due to the OS restriction (other
// platforms does not have this restriction), hotkey events must be handled
// on the "main thread". Therefore, in order to use this package properly,
// one must call the "(*Hotkey).Register" method on the main thread, and an
// OS app main event loop must be established. For self-contained applications,
// collaborating with the provided golang.design/x/hotkey/mainthread is possible.
// For applications based on other GUI frameworks, one has to use their provided
// ability to run the "(*Hotkey).Register" on the main thread. See the "./examples"
// folder for more examples.
package hotkey

import (
	"fmt"
	"runtime"
)

// Event represents a hotkey event
type Event struct{}

// Hotkey is a combination of modifiers and key to trigger an event
type Hotkey struct {
	platformHotkey

	mods []Modifier
	key  Key

	keydownIn  chan<- Event
	keydownOut <-chan Event
	keyupIn    chan<- Event
	keyupOut   <-chan Event
}

func New(mods []Modifier, key Key) *Hotkey {
	keydownIn, keydownOut := newEventChan()
	keyupIn, keyupOut := newEventChan()
	hk := &Hotkey{
		mods:       mods,
		key:        key,
		keydownIn:  keydownIn,
		keydownOut: keydownOut,
		keyupIn:    keyupIn,
		keyupOut:   keyupOut,
	}

	// Make sure the hotkey is unregistered when the created
	// hotkey is garbage collected.
	runtime.SetFinalizer(hk, func(x interface{}) {
		hk := x.(*Hotkey)
		hk.unregister()
		close(hk.keydownIn)
		close(hk.keyupIn)
	})
	return hk
}

// Register registers a combination of hotkeys. If the hotkey has
// registered. This function will invalidates the old registration
// and overwrites its callback.
//
// For macOS, this method must be called on the main thread, and
// an OS main event loop also must be running. This can be done when
// collaborating with the golang.design/x/hotkey/mainthread package.
func (hk *Hotkey) Register() error { return hk.register() }

// Keydown returns a channel that receives a signal when the hotkey is triggered.
func (hk *Hotkey) Keydown() <-chan Event { return hk.keydownOut }

// Keyup returns a channel that receives a signal when the hotkey is released.
//
// Platform specific details:
// - On Linux (X11), When AutoRepeat is enabled in the X server, the Keyup
//   is triggered automatically and continuously as Keydown continues.
func (hk *Hotkey) Keyup() <-chan Event { return hk.keyupOut }

// Unregister unregisters the hotkey.
func (hk *Hotkey) Unregister() error {
	err := hk.unregister()
	if err != nil {
		return err
	}

	// Reset a new event channel.
	close(hk.keydownIn)
	close(hk.keyupIn)
	hk.keydownIn, hk.keydownOut = newEventChan()
	hk.keyupIn, hk.keyupOut = newEventChan()
	return nil
}

// String returns a string representation of the hotkey.
func (hk *Hotkey) String() string {
	s := fmt.Sprintf("%v", hk.key)
	for _, mod := range hk.mods {
		s += fmt.Sprintf("+%v", mod)
	}
	return s
}

// newEventChan returns a sender and a receiver of a buffered channel
// with infinite capacity.
func newEventChan() (chan<- Event, <-chan Event) {
	in, out := make(chan Event), make(chan Event)

	go func() {
		var q []Event

		for {
			e, ok := <-in
			if !ok {
				close(out)
				return
			}
			q = append(q, e)
			for len(q) > 0 {
				select {
				case out <- q[0]:
					q[0] = Event{}
					q = q[1:]
				case e, ok := <-in:
					if ok {
						q = append(q, e)
						break
					}
					for _, e := range q {
						out <- e
					}
					close(out)
					return
				}
			}
		}
	}()
	return in, out
}
