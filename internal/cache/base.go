package cache

import (
	"os"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
)

const cacheCleanCooldown = 10 * time.Second

var (
	ValidDuration     = 1 * time.Minute
	cleanTaskInterval = ValidDuration / 2

	lastClean                     time.Time
	skippedCleanWithCanvasRefresh = false

	// cachedNow is the clock sample setAlive uses. Reading the real clock there
	// is an expensive operation and setAlive is called per-object per paint.
	// cachedNow is updated on Clean, which should be called once per frame.
	// Atomic because tests (and Resize/Refresh from goroutines in non-migrated apps)
	// reach setAlive off the main thread while the draw loop is in Clean.
	cachedNow atomic.Int64

	// testing purpose only
	timeNow = time.Now
)

func init() {
	if t, err := time.ParseDuration(os.Getenv("FYNE_CACHE")); err == nil {
		ValidDuration = t
		cleanTaskInterval = ValidDuration / 2
	}
	refreshNow()
}

// refreshNow samples the clock for setAlive and returns the sample.
func refreshNow() time.Time {
	now := timeNow()
	cachedNow.Store(now.UnixNano())
	return now
}

// Clean run cache clean task, it should be called on paint events.
func Clean(canvasRefreshed bool) {
	now := refreshNow()
	// do not run clean task too fast
	if now.Sub(lastClean) < cacheCleanCooldown {
		if canvasRefreshed {
			skippedCleanWithCanvasRefresh = true
		}
		return
	}
	if skippedCleanWithCanvasRefresh {
		skippedCleanWithCanvasRefresh = false
		canvasRefreshed = true
	}
	if !canvasRefreshed && now.Sub(lastClean) < cleanTaskInterval {
		return
	}
	destroyExpiredSvgs(now)
	destroyExpiredFontMetrics(now)
	destroyExpiredBlurKernels(now)
	if canvasRefreshed {
		// Destroy renderers on canvas refresh to avoid flickering screen.
		destroyExpiredRenderers(now)
		// canvases cache should be invalidated only on canvas refresh, otherwise there wouldn't
		// be a way to recover them later
		destroyExpiredCanvases(now)
	}
	lastClean = refreshNow()
}

// CleanCanvas performs a complete remove of all the objects that belong to the specified
// canvas. Usually used to free all objects from a closing windows.
func CleanCanvas(canvas fyne.Canvas) {
	canvases.Range(func(obj fyne.CanvasObject, cinfo *canvasInfo) bool {
		if cinfo.canvas != canvas {
			return true
		}

		canvases.Delete(obj)

		wid, ok := obj.(fyne.Widget)
		if !ok {
			return true
		}
		rinfo, ok := renderers.LoadAndDelete(wid)
		if !ok {
			return true
		}
		rinfo.renderer.Destroy()
		overrides.Delete(wid)
		return true
	})
}

// ResetThemeCaches clears all the svg and text size cache maps
func ResetThemeCaches() {
	svgs.Clear()
	ClearFontMetrics()
}

// destroyExpiredCanvases deletes objects from the canvases cache.
func destroyExpiredCanvases(now time.Time) {
	canvases.Range(func(obj fyne.CanvasObject, cinfo *canvasInfo) bool {
		if cinfo.isExpired(now) {
			canvases.Delete(obj)
			// A widget's override goes with its renderer, but a canvas
			// primitive has none, so its override goes when it leaves
			// every canvas. Otherwise each discarded themed text or
			// rectangle stays in overrides, and alive, for good.
			if _, ok := obj.(fyne.Widget); !ok {
				overrides.Delete(obj)
			}
		}
		return true
	})
}

// destroyExpiredRenderers deletes the renderer from the cache and calls
// renderer.Destroy()
func destroyExpiredRenderers(now time.Time) {
	renderers.Range(func(wid fyne.Widget, rinfo *rendererInfo) bool {
		if rinfo.isExpired(now) {
			forgetOverrides(rinfo.renderer.Objects())
			rinfo.renderer.Destroy()
			overrides.Delete(wid)
			renderers.Delete(wid)
		}
		return true
	})
}

// forgetOverrides drops the theme overrides of the primitives a renderer
// drew, through any containers. They were themed with the widget, and have
// no renderer of their own to take their overrides away, so without this
// each discarded themed text or rectangle stays in overrides, and alive,
// for good. A widget's own override goes with its own renderer.
func forgetOverrides(objs []fyne.CanvasObject) {
	for _, o := range objs {
		switch c := o.(type) {
		case fyne.Widget:
		case *fyne.Container:
			overrides.Delete(c)
			forgetOverrides(c.Objects)
		default:
			overrides.Delete(o)
		}
	}
}

type expiringCache struct {
	expires atomic.Int64 // unix nanos, so setAlive is a load and an add
}

// isExpired check if the cache data is expired.
func (c *expiringCache) isExpired(now time.Time) bool {
	return c.expires.Load() < now.UnixNano()
}

// setAlive updates expiration time.
func (c *expiringCache) setAlive() {
	c.expires.Store(cachedNow.Load() + ValidDuration.Nanoseconds())
}
