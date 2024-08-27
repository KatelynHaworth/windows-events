package windows_events

import "C"
import (
	"encoding/xml"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// EvtSubscribeToFutureEvents instructs the
	// subscription to only receive events that occur
	// after the subscription has been made
	EvtSubscribeToFutureEvents = 1

	// EvtSubscribeStartAtOldestRecord instructs the
	// subscription to receive all events (past and future)
	// that match the query
	EvtSubscribeStartAtOldestRecord = 2

	// evtSubscribeActionError defines a action
	// code that may be received by the winAPICallback.
	// ActionError defines that an internal error occurred
	// while obtaining an event for the callback
	evtSubscribeActionError = 0

	// evtSubscribeActionDeliver defines a action
	// code that may be received by the winAPICallback.
	// ActionDeliver defines that the internal API was
	// successful in obtaining an event that matched
	// the subscription query
	evtSubscribeActionDeliver = 1

	// evtRenderEventXML instructs procEvtRender
	// to render the event details as a XML string
	evtRenderEventXML = 1
)

var (
	modwevtapi = windows.NewLazySystemDLL("wevtapi.dll")

	procEvtSubscribe = modwevtapi.NewProc("EvtSubscribe")
	procEvtRender    = modwevtapi.NewProc("EvtRender")
	procEvtClose     = modwevtapi.NewProc("EvtClose")
)

// EventCallback defines the function
// layout required to receive events
type EventCallback func(event *Event)

// EventSubscription is a subscription to
// Windows Events, it defines details about the
// subscription including the channel and query
type EventSubscription struct {
	Channel         string
	Query           string
	SubscribeMethod int
	Errors          chan error
	Callback        EventCallback

	winAPIHandle windows.Handle
}

// Create will setup an event subscription in the
// windows kernel with the provided channel and
// event query
func (evtSub *EventSubscription) Create() error {
	if evtSub.winAPIHandle != 0 {
		return fmt.Errorf("windows_events: subscription already created in kernel")
	}

	winChannel, err := windows.UTF16PtrFromString(evtSub.Channel)
	if err != nil {
		return fmt.Errorf("windows_events: bad channel name: %s", err)
	}

	winQuery, err := windows.UTF16PtrFromString(evtSub.Query)
	if err != nil {
		return fmt.Errorf("windows_events: bad query string: %s", err)
	}

	handle, _, err := procEvtSubscribe.Call(
		0,
		0,
		uintptr(unsafe.Pointer(winChannel)),
		uintptr(unsafe.Pointer(winQuery)),
		0,
		0,
		syscall.NewCallback(evtSub.winAPICallback),
		uintptr(evtSub.SubscribeMethod),
	)

	if handle == 0 {
		return fmt.Errorf("windows_events: failed to subscribe to events: %s", err)
	}

	evtSub.winAPIHandle = windows.Handle(handle)
	return nil
}

// Close tells the windows kernel to let go
// of the event subscription handle as we
// are now done with it
func (evtSub *EventSubscription) Close() error {
	if evtSub.winAPIHandle == 0 {
		return fmt.Errorf("windows_events: no subscription to close")
	}

	if returnCode, _, err := procEvtClose.Call(uintptr(evtSub.winAPIHandle)); returnCode == 0 {
		return fmt.Errorf("windows_events: encountered error while closing event handle: %s", err)
	}

	evtSub.winAPIHandle = 0
	return nil
}

// winAPICallback receives the callback from the windows
// kernel when an event matching the query and channel is
// received. It will query the kernel to get the event rendered
// as a XML string, the XML string is then unmarshaled to an
// `Event` and the custom callback invoked
func (evtSub *EventSubscription) winAPICallback(action, userContext, event uintptr) uintptr {
	switch action {
	case evtSubscribeActionError:
		evtSub.Errors <- fmt.Errorf("windows_events: encountered error during callback: Win32 Error %x", uint16(event))

	case evtSubscribeActionDeliver:
		// See https://learn.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtrender
		renderSpace := make([]uint16, 4096)
		bufferSize := (len(renderSpace) - 1) * 2 // The buffer size in bytes, allowing for extra null-terminator
		bufferUsed := uint32(0)                  // A Windows DWORD (32 bit)
		propertyCount := uint32(0)               // A Windows DWORD (32 bit)

		returnCode, _, err := procEvtRender.Call(0, event, evtRenderEventXML, uintptr(bufferSize), uintptr(unsafe.Pointer(&renderSpace[0])),
			uintptr(unsafe.Pointer(&bufferUsed)), uintptr(unsafe.Pointer(&propertyCount)))

		if returnCode == 0 {
			evtSub.Errors <- fmt.Errorf("windows_event: failed to render event data: %s", err)
		} else {
			dataParsed := new(Event)
			err := xml.Unmarshal([]byte(windows.UTF16ToString(renderSpace)), dataParsed)

			if err != nil {
				evtSub.Errors <- fmt.Errorf("windows_event: failed to unmarshal event xml: %s", err)
			} else {
				evtSub.Callback(dataParsed)
			}
		}

	default:
		evtSub.Errors <- fmt.Errorf("windows_events: encountered error during callback: unsupported action code %x", uint16(action))
	}

	return 0
}
