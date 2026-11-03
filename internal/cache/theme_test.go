package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fyne.io/fyne/v2"
)

// dummyObject is a canvas primitive: an object with no renderer of its own.
type dummyObject struct {
	fyne.CanvasObject
}

// themedWidget draws the objects it is given, as a widget's renderer does.
type themedWidget struct {
	fyne.Widget
	objects []fyne.CanvasObject
}

func (w *themedWidget) CreateRenderer() fyne.WidgetRenderer {
	return &dummyWidgetRenderer{widget: &dummyWidget{}, objects: w.objects}
}

func TestTheOverridesOfAnExpiredRenderersPrimitivesAreForgotten(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	text := &dummyObject{}
	w := &themedWidget{objects: []fyne.CanvasObject{&fyne.Container{Objects: []fyne.CanvasObject{text}}}}
	OverrideTheme(w, nil)
	assert.Equal(t, 2, overrides.Len(), "the widget and the text inside its renderer's container")

	tm.setTime(12, 0)
	lastClean = tm.createTime(11, 0)
	Clean(true)
	assert.Zero(t, renderers.Len())
	assert.Zero(t, overrides.Len(), "the text outlived the renderer that drew it")
}

func TestTheOverrideOfAPrimitiveThatLeftTheCanvasIsForgotten(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	text := &dummyObject{}
	OverrideTheme(text, nil)
	SetCanvasForObject(text, &dummyCanvas{}, nil)
	assert.Equal(t, 1, overrides.Len())

	tm.setTime(12, 0)
	lastClean = tm.createTime(11, 0)
	Clean(true)
	assert.Zero(t, canvases.Len())
	assert.Zero(t, overrides.Len(), "the text outlived its place on the canvas")
}
