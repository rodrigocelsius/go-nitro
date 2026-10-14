package nitro

import "sync"

// flightGroup deduplicates concurrent calls with the same key: the first
// caller runs fn while the others block and receive its result — preventing
// cache-miss stampedes against the backing store.
//
// Minimal in-package equivalent of golang.org/x/sync/singleflight, kept here
// so the core module stays dependency-free.
type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	wg  sync.WaitGroup
	val any
	err error
}

// Do executes fn, deduplicating concurrent invocations for key.
// If another call with the same key is in flight, Do waits for it and
// returns its result instead of running fn again.
func (g *flightGroup) Do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*flightCall)
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &flightCall{}
	c.wg.Add(1)
	g.calls[key] = c
	g.mu.Unlock()

	defer func() {
		// A panicking fn must not wedge waiters forever.
		if r := recover(); r != nil {
			c.err = fmtPanic(r)
			g.finish(key, c)
			panic(r)
		}
	}()

	c.val, c.err = fn()
	g.finish(key, c)
	return c.val, c.err
}

func (g *flightGroup) finish(key string, c *flightCall) {
	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	c.wg.Done()
}

func fmtPanic(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return panicError{r}
}

type panicError struct{ v any }

func (e panicError) Error() string {
	return "nitro: panic during cached load"
}
