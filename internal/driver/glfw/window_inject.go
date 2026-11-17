package glfw

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/internal/async"
)

var _ desktop.InputInjector = (*window)(nil)

func (w *window) InjectMouseMove(pos fyne.Position) {
	w.runInjected(func() {
		scale := w.canvas.Scale()
		w.newMousePosX = float64(pos.X * scale)
		w.newMousePosY = float64(pos.Y * scale)
		w.mousePosUpdateProcessed = false
		w.ensurePositionProcessed()
	})
}

func (w *window) InjectMouseButton(button desktop.MouseButton, pressed bool, modifier fyne.KeyModifier) {
	w.runInjected(func() {
		w.processMouseClicked(button, injectedAction(pressed), modifier)
	})
}

func (w *window) InjectScroll(dx, dy float64) {
	w.runInjected(func() {
		w.processMouseScrolled(dx, dy)
	})
}

func (w *window) InjectKey(key fyne.KeyName, pressed bool, modifier fyne.KeyModifier) {
	w.runInjected(func() {
		ascii := fyne.KeyUnknown
		if len(key) == 1 && key[0] >= 'A' && key[0] <= 'Z' {
			ascii = key
		}
		w.driver.currentKeyModifiers = modifier
		w.processKeyPressed(key, ascii, 0, injectedAction(pressed), modifier)
	})
}

func (w *window) InjectChar(r rune) {
	w.runInjected(func() {
		w.processCharInput(r)
	})
}

// runInjected calls fn on the main goroutine, directly when already there so
// a caller inside fyne.DoAndWait does not wait on the queue it is draining.
func (w *window) runInjected(fn func()) {
	if async.IsMainGoroutine() {
		fn()
		return
	}
	runOnMain(fn)
}

func injectedAction(pressed bool) action {
	if pressed {
		return press
	}
	return release
}
