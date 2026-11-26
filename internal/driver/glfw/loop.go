package glfw

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/internal/app"
	"fyne.io/fyne/v2/internal/async"
	"fyne.io/fyne/v2/internal/cache"
	"fyne.io/fyne/v2/internal/driver/common"
	"fyne.io/fyne/v2/internal/goos"
	"fyne.io/fyne/v2/internal/painter"
	"fyne.io/fyne/v2/internal/scale"
)

type funcData struct {
	f    func()
	done chan struct{} // Zero allocation signalling channel
}

// channel for queuing functions on the main thread
var (
	funcQueue = newMainQueue()
	running   atomic.Bool
)

// mainQueue carries functions to the main loop until the loop stops.
//
// Checking whether the loop has stopped and queuing a function must be one
// step: a function queued after the loop's last look at the queue, but before
// it said it had stopped, was never run and never answered, so a caller of
// DoAndWait waited forever. The lifecycle's event goroutine was such a caller
// when it handed over the OnStopped hook, and Run then waited on that
// goroutine in WaitForEvents, so the process never exited.
type mainQueue struct {
	mu      sync.RWMutex
	stopped bool
	ch      *async.UnboundedChan[funcData]
}

func newMainQueue() *mainQueue {
	return &mainQueue{ch: async.NewUnboundedChan[funcData]()}
}

// post queues fn, and reports false, queuing nothing, once stop has run.
func (q *mainQueue) post(fn funcData) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.stopped {
		return false
	}
	// The channel's own goroutine always takes from In, so this does not
	// hold the lock for long.
	q.ch.In() <- fn
	return true
}

func (q *mainQueue) out() <-chan funcData { return q.ch.Out() }

// stop refuses further functions, then answers every caller still waiting on
// one queued before, without running it: GLFW has terminated, and what was
// queued may touch it (a second Quit closing the windows again panics). A
// function refused after this runs on its caller, as before the queue had a
// stop of its own.
func (q *mainQueue) stop() {
	q.mu.Lock()
	q.stopped = true
	q.mu.Unlock()

	q.ch.Close()
	// Close hands what it holds to Out and then closes it; reading until it
	// does catches every function queued before stopped was set.
	for f := range q.ch.Out() {
		if f.done != nil {
			f.done <- struct{}{}
		}
	}
}

// Arrange that main.main runs on main thread.
func init() {
	runtime.LockOSThread()
	async.SetMainGoroutine()
}

// force a function f to run on the main thread
func runOnMain(f func()) {
	runOnMainWithWait(f, true)
}

// force a function f to run on the main thread and specify if we should wait for it to return
func runOnMainWithWait(f func(), wait bool) {
	// If we are on main before app run just execute - otherwise add it to the main queue and wait.
	// We also need to run it as-is if the app is in the process of shutting down as the queue will be stopped.
	if !running.Load() && async.IsMainGoroutine() {
		f()
		return
	}

	if !wait {
		if !funcQueue.post(funcData{f: f}) {
			f()
		}
		return
	}

	done := common.DonePool.Get()
	defer common.DonePool.Put(done)
	if !funcQueue.post(funcData{f: f, done: done}) {
		f()
		return
	}
	<-done
}

func decideRepaint(visible, ready bool, checkDirtyAndClear func() bool) bool {
	return visible && ready && checkDirtyAndClear()
}

func (d *gLDriver) drawSingleFrame() {
	refreshed := false
	for _, win := range d.AllWindows() {
		w, _ := win.(*window)
		if w.closing {
			continue
		}

		if decideRepaint(w.visible, w.frame.ready(), w.canvas.CheckDirtyAndClear) {
			w.RunWithContext(func() {
				if w.driver.repaintWindow(w) {
					refreshed = true
				}
			})
			w.updateAccessibility()
		} else {
			w.markCacheAlive()
		}
	}
	cache.Clean(refreshed)
}

func (w *window) markCacheAlive() {
	threshold := time.Now().Add(10*time.Second - cache.ValidDuration)
	if w.lastWalkedTime.Before(threshold) {
		w.canvas.WalkTrees(nil, func(node *common.RenderCacheNode, _ fyne.Position) {
			_ = cache.GetCanvasForObject(node.Obj())
			if wid, ok := node.Obj().(fyne.Widget); ok {
				_, _ = cache.CachedRenderer(wid)
			}
		})
		w.lastWalkedTime = time.Now()
	}
}

func (*gLDriver) applyThemeToWindow(w fyne.Window) {
	if win, ok := w.(*window); ok {
		win.setDarkMode()
	}
}

func (d *gLDriver) runGL() {
	if !running.CompareAndSwap(false, true) {
		return // Run was called twice.
	}

	d.init()
	if d.trayStart != nil {
		d.trayStart()
	}

	fyne.CurrentApp().Settings().AddListener(func(set fyne.Settings) {
		painter.ClearFontCache()
		cache.ResetThemeCaches()
		app.ApplySettingsWithCallback(set, fyne.CurrentApp(), func(w fyne.Window) {
			d.applyThemeToWindow(w)
			c, ok := w.Canvas().(*glCanvas)
			if !ok {
				return
			}
			c.applyThemeOutOfTreeObjects()
			c.reloadScale()
		})
	})

	if f := fyne.CurrentApp().Lifecycle().(*app.Lifecycle).OnStarted(); f != nil {
		f()
	}

	eventTick := time.NewTicker(time.Second / 60)
	for {
		select {
		case <-d.done:
			eventTick.Stop()
			d.Terminate()

			// as we are shutting down make sure we drain the pending funcQueue and close it out.
			// This comes before the OnStopped hook is queued: the event goroutine
			// then always finds the queue stopped and runs the hook itself.
			funcQueue.stop()

			l, _ := fyne.CurrentApp().Lifecycle().(*app.Lifecycle)
			if f := l.OnStopped(); f != nil {
				l.QueueEvent(f)
			}
			return
		case f := <-funcQueue.out():
			f.f()
			if f.done != nil {
				f.done <- struct{}{}
			}
		case <-eventTick.C:
			d.pollEvents()
			for i := 0; i < len(d.windows); i++ {
				w, _ := d.windows[i].(*window)
				w.ensurePositionProcessed()

				if w.viewport == nil {
					continue
				}

				if w.viewport.ShouldClose() {
					d.destroyWindow(w, i)
					i-- // Trailing windows are moved forward one step.
					continue
				}

				expand := w.shouldExpand
				fullScreen := w.fullScreen

				if expand && !fullScreen {
					w.fitContent()
					shouldExpand := w.shouldExpand
					w.shouldExpand = false
					view := w.viewport

					if shouldExpand && runtime.GOOS != goos.JavaScript {
						view.SetSize(w.shouldWidth, w.shouldHeight)
					}
				}
			}

			d.animation.TickAnimations()
			d.drawSingleFrame()
		}
	}
}

func (d *gLDriver) destroyWindow(w *window, index int) {
	w.visible = false
	w.viewport.Destroy()
	w.destroy(d)

	if index < len(d.windows)-1 {
		copy(d.windows[index:], d.windows[index+1:])
	}
	d.windows[len(d.windows)-1] = nil
	d.windows = d.windows[:len(d.windows)-1]

	if len(d.windows) == 0 {
		d.Quit()
	}
}

func (*gLDriver) repaintWindow(w *window) bool {
	canvas := w.canvas
	freed := false
	if canvas.EnsureMinSize() {
		w.shouldExpand = true
	}
	freed = canvas.FreeDirtyTextures() > 0

	updateGLContext(w)
	canvas.paint(canvas.Size())

	view := w.viewport
	visible := w.visible

	if view != nil && visible {
		w.frame.requestFrame()
		view.SwapBuffers()
	}

	// mark that we have walked the window and don't
	// need to walk it again to mark caches alive
	w.lastWalkedTime = time.Now()
	return freed
}

// refreshWindow requests that the specified window be redrawn
func refreshWindow(w *window) {
	w.canvas.SetDirty()
}

func updateGLContext(w *window) {
	canvas := w.canvas
	size := canvas.Size()

	// On X11 the framebuffer size is a live query and may differ from the canvas
	// until the resize callback has run, so pass the real drawable size.
	fbWidth, fbHeight := w.viewport.GetFramebufferSize()
	if fbWidth <= 0 || fbHeight <= 0 {
		fbWidth = int(float32(scale.ToScreenCoordinate(canvas, size.Width)) * canvas.texScale)
		fbHeight = int(float32(scale.ToScreenCoordinate(canvas, size.Height)) * canvas.texScale)
	}

	canvas.Painter().SetFrameBufferScale(canvas.texScale)
	canvas.Painter().SetOutputSize(fbWidth, fbHeight)
}
