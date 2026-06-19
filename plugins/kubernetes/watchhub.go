package kubernetes

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// feedKey identifies one upstream watch; single-object feeds set FieldSelector to
// metadata.name so the upstream streams only that object.
type feedKey struct {
	GVR           schema.GroupVersionResource
	Namespace     string
	FieldSelector string
	LabelSelector string
}

// watchHub runs one upstream watch per feedKey, fanned out to N subscribers and
// ref-counted, with resourceVersion resume. It is bounded streaming (no per-kind
// informer cache) to suit a many-connection gateway. Close cancels ctx to unblock
// every upstream watch deterministically.
type watchHub struct {
	s      *Session
	ctx    context.Context
	cancel context.CancelFunc

	watchFn func(ctx context.Context, key feedKey, opts metav1.ListOptions) (watch.Interface, error)

	mu    sync.Mutex
	feeds map[feedKey]*feed
}

func newWatchHub(s *Session) *watchHub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &watchHub{s: s, ctx: ctx, cancel: cancel, feeds: map[feedKey]*feed{}}
	h.watchFn = h.dynamicWatch
	return h
}

func (h *watchHub) dynamicWatch(ctx context.Context, key feedKey, opts metav1.ListOptions) (watch.Interface, error) {
	ri := h.s.Dynamic().Resource(key.GVR)
	if key.Namespace != "" {
		return ri.Namespace(key.Namespace).Watch(ctx, opts)
	}
	return ri.Watch(ctx, opts)
}

func (h *watchHub) Close() {
	if h == nil {
		return
	}
	h.cancel()
}

// Subscribe attaches to key's feed, starting the upstream watch on the first
// subscriber and stopping it when the last unsubscribe runs.
func (h *watchHub) Subscribe(key feedKey) (<-chan watch.Event, func()) {
	h.mu.Lock()
	f := h.feeds[key]
	if f == nil {
		f = newFeed(h, key)
		h.feeds[key] = f
		go f.run()
	}
	ch, id := f.add()
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if f.remove(id) == 0 {
			f.stop()
			delete(h.feeds, key)
		}
	}
}

const watchFeedBuffer = 128

type feed struct {
	hub    *watchHub
	key    feedKey
	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	subs map[int]chan watch.Event
	seq  int
}

func newFeed(h *watchHub, key feedKey) *feed {
	ctx, cancel := context.WithCancel(h.ctx)
	return &feed{hub: h, key: key, ctx: ctx, cancel: cancel, subs: map[int]chan watch.Event{}}
}

func (f *feed) add() (<-chan watch.Event, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.seq
	f.seq++
	ch := make(chan watch.Event, watchFeedBuffer)
	f.subs[id] = ch
	return ch, id
}

// remove drops a subscriber and returns the remaining count; the caller holds
// hub.mu so the count→stop decision can't race a new subscriber.
func (f *feed) remove(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.subs[id]; ok {
		delete(f.subs, id)
		close(ch)
	}
	return len(f.subs)
}

func (f *feed) stop() { f.cancel() }

// broadcast drops the oldest event for a subscriber whose buffer is full rather
// than blocking the shared upstream; the UI re-syncs on the next event or reconnect.
func (f *feed) broadcast(ev watch.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range f.subs {
		select {
		case ch <- ev:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// run establishes the upstream watch and resumes from the last resourceVersion
// across the apiserver's periodic closes; a 410/Expired drops the RV and restarts
// from current.
func (f *feed) run() {
	var lastRV string
	backoff := 250 * time.Millisecond
	for {
		if f.ctx.Err() != nil {
			return
		}
		w, err := f.hub.watchFn(f.ctx, f.key, metav1.ListOptions{
			FieldSelector:       f.key.FieldSelector,
			LabelSelector:       f.key.LabelSelector,
			ResourceVersion:     lastRV,
			AllowWatchBookmarks: true,
		})
		if err != nil {
			if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
				lastRV = ""
				continue
			}
			if !f.sleep(backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = 250 * time.Millisecond
		lastRV = f.drain(w, lastRV)
		w.Stop()
	}
}

func (f *feed) drain(w watch.Interface, lastRV string) string {
	for {
		select {
		case <-f.ctx.Done():
			return lastRV
		case ev, ok := <-w.ResultChan():
			if !ok {
				return lastRV
			}
			if ev.Type == watch.Error {
				return "" // stale RV: restart from current
			}
			if rv := resourceVersionOf(ev.Object); rv != "" {
				lastRV = rv
			}
			if ev.Type == watch.Bookmark {
				continue
			}
			f.broadcast(ev)
		}
	}
}

func (f *feed) sleep(d time.Duration) bool {
	select {
	case <-f.ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func nextBackoff(d time.Duration) time.Duration {
	const maxBackoff = 5 * time.Second
	if d *= 2; d > maxBackoff {
		return maxBackoff
	}
	return d
}

func resourceVersionOf(o any) string {
	accessor, err := meta.Accessor(o)
	if err != nil {
		return ""
	}
	return accessor.GetResourceVersion()
}
