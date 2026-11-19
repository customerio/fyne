package desktop

// FrameCapturer is implemented by canvases that can copy their last painted
// frame as raw pixels, which is cheap enough to call many times a second.
// It is intended for screen recording and automated testing.
//
// Since: 2.8
type FrameCapturer interface {
	// CaptureFrame copies the last painted frame into buf, growing it when it
	// is too small, and returns the pixels with the frame's size in pixels.
	// The pixels are RGBA, four bytes each, with the bottom row first.
	CaptureFrame(buf []uint8) (pix []uint8, width, height int)
}
