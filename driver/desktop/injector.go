package desktop

import "fyne.io/fyne/v2"

// InputInjector is implemented by windows that can receive synthesised input.
// Injected events take the same path as events from the operating system, so
// hit-testing, focus, hover, drag and double-tap handling all apply.
// It is intended for automated testing and accessibility tooling.
// Each method may be called from any goroutine, including inside [fyne.Do].
//
// Since: 2.8
type InputInjector interface {
	// InjectMouseMove moves the pointer to a position in canvas coordinates.
	InjectMouseMove(pos fyne.Position)

	// InjectMouseButton presses or releases a button at the current pointer position.
	InjectMouseButton(button MouseButton, pressed bool, modifier fyne.KeyModifier)

	// InjectScroll scrolls at the current pointer position by offsets in the
	// units the operating system reports for a wheel or trackpad.
	InjectScroll(dx, dy float64)

	// InjectKey presses or releases a key, delivered to the focused object or the canvas.
	InjectKey(key fyne.KeyName, pressed bool, modifier fyne.KeyModifier)

	// InjectChar types a character, delivered to the focused object or the canvas.
	InjectChar(r rune)
}
