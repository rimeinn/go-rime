package rime

import (
	"sync"
	"sync/atomic"

	"github.com/rimeinn/go-rime/abi"
)

// Notification is a copy of a librime event. SessionID is zero for deployment
// events. Type and Value retain the native message format.
type Notification struct {
	SessionID uintptr
	Type      string
	Value     string
}

type notificationHub struct {
	mu          sync.Mutex
	subscribers map[chan Notification]struct{}
}

var currentNotifications atomic.Pointer[notificationHub]

// A single trampoline is reused for the process. It never takes the engine
// lock, since native calls holding that lock can emit notifications.
var notificationCallback = sync.OnceValue(func() uintptr {
	return abi.NewNotificationCallback(func(_ uintptr, id abi.SessionID, typ, value *byte) {
		hub := currentNotifications.Load()
		if hub == nil {
			return
		}
		event := Notification{SessionID: id, Type: abi.GoString(typ), Value: abi.GoString(value)}
		hub.mu.Lock()
		defer hub.mu.Unlock()
		for ch := range hub.subscribers {
			select {
			case ch <- event:
			default:
			}
		}
	})
})

func (e *Engine) registerNotifications() {
	if e.api.SetNotificationHandler == nil {
		return
	}
	callback := notificationCallback()
	if callback == 0 {
		return
	}
	e.notify = &notificationHub{subscribers: make(map[chan Notification]struct{})}
	currentNotifications.Store(e.notify)
	e.api.SetNotificationHandler(callback, 0)
}

func (e *Engine) unregisterNotifications() {
	if e.notify == nil {
		return
	}
	e.api.SetNotificationHandler(0, 0)
	currentNotifications.Store(nil)
	e.notify.mu.Lock()
	defer e.notify.mu.Unlock()
	for ch := range e.notify.subscribers {
		close(ch)
	}
	clear(e.notify.subscribers)
}

// Subscribe receives future native notifications on a channel buffered for 32
// events. An event is dropped for a subscriber whose buffer is full. Call the
// returned cancellation function to remove the subscriber and close its channel.
// Cancellation is idempotent, and Close cancels all subscriptions.
func (e *Engine) Subscribe() (<-chan Notification, func(), error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.checkOpenLocked(); err != nil {
		return nil, nil, err
	}
	hub := e.notify
	if hub == nil {
		return nil, nil, unsupported("set_notification_handler")
	}
	ch := make(chan Notification, 32)
	hub.mu.Lock()
	hub.subscribers[ch] = struct{}{}
	hub.mu.Unlock()
	return ch, func() {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		if _, ok := hub.subscribers[ch]; ok {
			delete(hub.subscribers, ch)
			close(ch)
		}
	}, nil
}
