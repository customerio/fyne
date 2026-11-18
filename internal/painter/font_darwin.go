//go:build darwin

package painter

import (
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
)

// systemFontLocation is the file a system fallback face was loaded from.
func systemFontLocation(f *font.Font) (fontscan.Location, bool) {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()
	if fm == nil {
		return fontscan.Location{}, false
	}
	loc := fm.FontLocation(f)
	return loc, loc.File != ""
}
