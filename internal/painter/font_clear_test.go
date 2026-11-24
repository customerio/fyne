package painter

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-text/typesetting/font"
	"github.com/stretchr/testify/assert"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// TestClearFontCacheLetsShapedFacesGo shapes text with a face and clears the
// font cache: nothing should hold the face after that. The shaper kept a
// shape plan per face it had shaped with, so every clear, as each test app
// makes, left a parsed font set alive for the life of the process.
func TestClearFontCacheLetsShapedFacesGo(t *testing.T) {
	ClearFontCache()
	var freed atomic.Bool
	func() {
		face := loadMeasureFont(theme.DefaultTextFont())
		if face == nil {
			t.Fatal("failed to load the bundled text font")
		}
		runtime.SetFinalizer(face, func(any) { freed.Store(true) })
		faces := &dynamicFontMap{family: "test", faces: []*font.Face{face}, fixed: 1}
		size, _ := MeasureString(faces, "shaped once", 14, fyne.TextStyle{})
		assert.Positive(t, size.Width)
	}()

	ClearFontCache()
	for i := 0; i < 20 && !freed.Load(); i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, freed.Load(), "a face shaped before ClearFontCache is still reachable after it")
}
