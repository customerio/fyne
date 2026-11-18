//go:build !no_glfw && !mobile

package glfw

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/stretchr/testify/assert"
)

func TestWindow_InjectInput(t *testing.T) {
	t.Run("given a tappable under the pointer, an injected click should tap it at the injected position", func(t *testing.T) {
		w := createWindow("Test")
		o := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
		w.SetContent(o)
		w.Resize(fyne.NewSize(100, 100))
		repaintWindow(w)

		var inj desktop.InputInjector = w.window
		inj.InjectMouseMove(fyne.NewPos(30, 40))
		inj.InjectMouseButton(desktop.MouseButtonPrimary, true, 0)
		inj.InjectMouseButton(desktop.MouseButtonPrimary, false, 0)

		runOnMain(func() {
			e, _ := o.popTapEvent().(*fyne.PointEvent)
			if assert.NotNil(t, e) {
				assert.Equal(t, fyne.NewPos(30, 40), e.AbsolutePosition)
			}
		})
	})

	t.Run("given the OS cursor elsewhere, an injected click should tap at the injected position", func(t *testing.T) {
		w := createWindow("Test")
		o := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
		w.SetContent(o)
		w.Resize(fyne.NewSize(100, 100))
		repaintWindow(w)
		w.cursorPosition = func() (float64, float64, bool) { return 5, 5, true }

		w.InjectMouseMove(fyne.NewPos(30, 40))
		w.InjectMouseButton(desktop.MouseButtonPrimary, true, 0)
		w.InjectMouseButton(desktop.MouseButtonPrimary, false, 0)

		runOnMain(func() {
			e, _ := o.popTapEvent().(*fyne.PointEvent)
			if assert.NotNil(t, e) {
				assert.Equal(t, fyne.NewPos(30, 40), e.AbsolutePosition)
			}
		})
	})

	t.Run("given an overlay above the content, an injected click should reach the overlay and not the content", func(t *testing.T) {
		w := createWindow("Test")
		under := &tappableObject{Rectangle: canvas.NewRectangle(color.White)}
		over := &tappableObject{Rectangle: canvas.NewRectangle(color.Black)}
		w.SetContent(under)
		w.Resize(fyne.NewSize(100, 100))
		runOnMain(func() {
			w.window.Canvas().Overlays().Add(over)
		})
		repaintWindow(w)

		w.InjectMouseMove(fyne.NewPos(50, 50))
		w.InjectMouseButton(desktop.MouseButtonPrimary, true, 0)
		w.InjectMouseButton(desktop.MouseButtonPrimary, false, 0)

		runOnMain(func() {
			assert.NotNil(t, over.popTapEvent())
			assert.Nil(t, under.popTapEvent())
		})
	})

	t.Run("given a hoverable, an injected move should hover it and moving away should leave it", func(t *testing.T) {
		w := createWindow("Test")
		h := &hoverableObject{Rectangle: canvas.NewRectangle(color.White)}
		h.SetMinSize(fyne.NewSize(10, 10))
		w.SetContent(container.NewHBox(h, canvas.NewRectangle(color.Black)))
		w.Resize(fyne.NewSize(60, 20))
		repaintWindow(w)

		w.InjectMouseMove(fyne.NewPos(5, 5))
		runOnMain(func() {
			assert.NotNil(t, h.popMouseInEvent())
		})

		w.InjectMouseMove(fyne.NewPos(50, 5))
		runOnMain(func() {
			assert.NotNil(t, h.popMouseOutEvent())
		})
	})

	t.Run("given a scrollable under the pointer, an injected scroll should scroll it", func(t *testing.T) {
		w := createWindow("Test")
		s := &scrollable{Rectangle: canvas.NewRectangle(color.White)}
		w.SetContent(s)
		w.Resize(fyne.NewSize(100, 100))
		repaintWindow(w)

		w.InjectMouseMove(fyne.NewPos(50, 50))
		w.InjectScroll(0, -1)

		runOnMain(func() {
			e, _ := s.popScrollEvent().(*fyne.ScrollEvent)
			if assert.NotNil(t, e) {
				assert.Negative(t, e.Scrolled.DY)
			}
		})
	})

	t.Run("given nothing focused, injected keys and characters should reach the canvas handlers", func(t *testing.T) {
		w := createWindow("Test")
		w.SetContent(canvas.NewRectangle(color.White))
		var keys []fyne.KeyName
		var runes []rune
		runOnMain(func() {
			w.window.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { keys = append(keys, e.Name) })
			w.window.Canvas().SetOnTypedRune(func(r rune) { runes = append(runes, r) })
		})

		w.InjectKey(fyne.KeyEscape, true, 0)
		w.InjectKey(fyne.KeyEscape, false, 0)
		w.InjectChar('x')

		runOnMain(func() {
			assert.Equal(t, []fyne.KeyName{fyne.KeyEscape}, keys)
			assert.Equal(t, []rune{'x'}, runes)
		})
	})

	t.Run("given a caller already on the main goroutine, injecting should run in place without deadlocking", func(t *testing.T) {
		w := createWindow("Test")
		w.SetContent(canvas.NewRectangle(color.White))
		var runes []rune
		runOnMain(func() {
			w.window.Canvas().SetOnTypedRune(func(r rune) { runes = append(runes, r) })
			w.InjectChar('y')
			assert.Equal(t, []rune{'y'}, runes)
		})
	})
}
