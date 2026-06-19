package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func cmObj(name, rv string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: obj{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": obj{"name": name, "namespace": "default", "resourceVersion": rv},
	}}
}

// newTestHub builds a hub whose upstream watches come from a caller-controlled
// factory, recording the resourceVersion each (re)connect resumes from.
func newTestHub(t *testing.T, factory func(rv string) watch.Interface) (*watchHub, *[]string) {
	t.Helper()
	h := &watchHub{ctx: context.Background(), feeds: map[feedKey]*feed{}}
	ctx, cancel := context.WithCancel(context.Background())
	h.ctx, h.cancel = ctx, cancel
	t.Cleanup(cancel)
	var mu sync.Mutex
	resumes := []string{}
	h.watchFn = func(_ context.Context, _ feedKey, opts metav1.ListOptions) (watch.Interface, error) {
		mu.Lock()
		resumes = append(resumes, opts.ResourceVersion)
		mu.Unlock()
		return factory(opts.ResourceVersion), nil
	}
	return h, &resumes
}

func TestWatchHubFanOut(t *testing.T) {
	fw := watch.NewFakeWithChanSize(8, false)
	h, _ := newTestHub(t, func(string) watch.Interface { return fw })
	key := feedKey{GVR: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, Namespace: "default"}

	a, cancelA := h.Subscribe(key)
	b, cancelB := h.Subscribe(key)
	defer cancelA()
	defer cancelB()

	fw.Modify(cmObj("cfg", "2"))
	for i, ch := range []<-chan watch.Event{a, b} {
		select {
		case ev := <-ch:
			if ev.Type != watch.Modified {
				t.Fatalf("sub %d got %s", i, ev.Type)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("sub %d received no event (fan-out broken)", i)
		}
	}
}

func TestWatchHubRefcountStopsUpstream(t *testing.T) {
	stopped := make(chan struct{})
	fw := watch.NewFakeWithChanSize(1, false)
	h, _ := newTestHub(t, func(string) watch.Interface { return fw })
	key := feedKey{GVR: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}}

	_, cancel1 := h.Subscribe(key)
	_, cancel2 := h.Subscribe(key)
	h.mu.Lock()
	if len(h.feeds) != 1 {
		h.mu.Unlock()
		t.Fatal("two subscribers should share one feed")
	}
	f := h.feeds[key]
	h.mu.Unlock()
	go func() { <-f.ctx.Done(); close(stopped) }()

	cancel1()
	select {
	case <-stopped:
		t.Fatal("feed stopped while a subscriber remained")
	case <-time.After(100 * time.Millisecond):
	}
	cancel2()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("feed did not stop after the last subscriber left")
	}
	h.mu.Lock()
	if len(h.feeds) != 0 {
		t.Fatal("feed not removed from hub after last unsubscribe")
	}
	h.mu.Unlock()
}

// TestWatchHubResumesResourceVersion proves a transient watch close re-establishes
// from the last seen resourceVersion (no gap), and bookmark events advance it.
func TestWatchHubResumesResourceVersion(t *testing.T) {
	var mu sync.Mutex
	watchers := []*watch.FakeWatcher{}
	factory := func(string) watch.Interface {
		fw := watch.NewFakeWithChanSize(8, false)
		mu.Lock()
		watchers = append(watchers, fw)
		mu.Unlock()
		return fw
	}
	h, resumes := newTestHub(t, factory)
	key := feedKey{GVR: schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}}

	ch, cancel := h.Subscribe(key)
	defer cancel()

	waitFor := func(n int) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			mu.Lock()
			got := len(watchers)
			mu.Unlock()
			if got >= n {
				return
			}
			select {
			case <-deadline:
				t.Fatalf("expected %d upstream watch(es)", n)
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitFor(1)
	mu.Lock()
	first := watchers[0]
	mu.Unlock()
	first.Modify(cmObj("cfg", "5")) // advance RV to 5
	<-ch
	// A bookmark advances RV without emitting a row event.
	first.Action(watch.Bookmark, cmObj("cfg", "7"))
	time.Sleep(50 * time.Millisecond)
	first.Stop() // transient close → run() must reconnect from RV 7

	waitFor(2)
	if last := (*resumes)[len(*resumes)-1]; last != "7" {
		t.Fatalf("reconnect resumed from RV %q, want 7", last)
	}
}
