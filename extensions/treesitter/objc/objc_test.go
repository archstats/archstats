package objc

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyse(t *testing.T, files map[string]string) (map[string]*file.Results, map[string]*unit.Unit) {
	t.Helper()
	a := &analyzer{lp: createPack()}
	var all []*file.Results
	byName := map[string]*file.Results{}
	for p, src := range files {
		res := a.analyze(p, []byte(src))
		require.NotNil(t, res, p)
		res.Name = p
		res.Directory = p[:len(p)-len(p[lastSlash(p):])]
		for _, s := range res.Snippets {
			s.Component = res.Directory
		}
		all = append(all, res)
		byName[p] = res
	}
	(&apple.Linker{Root: t.TempDir()}).EditFileResults(all)
	units := map[string]*unit.Unit{}
	for _, fr := range all {
		for _, u := range fr.Units {
			if e, ok := units[u.ID]; ok {
				e.Markers = append(e.Markers, u.Markers...)
				continue
			}
			units[u.ID] = u
		}
	}
	return byName, units
}

func lastSlash(p string) int {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return i
		}
	}
	return 0
}

var sdweb = map[string]string{
	"SDWebImage/Core/SDWebImageManager.h": `#import <Foundation/Foundation.h>
#import "SDImageCache.h"
@protocol SDImageLoader <NSObject>
- (void)load;
@end
@interface SDWebImageManager : NSObject <SDImageLoader, NSCopying>
@property (nonatomic, strong) SDImageCache *imageCache;
+ (instancetype)sharedManager;
@end
`,
	"SDWebImage/Core/SDWebImageManager.m": `#import "SDWebImageManager.h"
@interface SDWebImageManager ()
@property (nonatomic) BOOL running;
@end
@implementation SDWebImageManager
- (void)load { [[SDImageCache sharedCache] clear]; }
+ (instancetype)sharedManager { return nil; }
@end
`,
	"SDWebImage/Cache/SDImageCache.h": `@interface SDImageCache : NSObject
+ (instancetype)sharedCache;
- (void)clear;
@end
`,
	"SDWebImage/Categories/UIView+WebCache.m": `#import "SDWebImageManager.h"
@implementation UIView (WebCache)
- (void)sd_setImage { [SDWebImageManager sharedManager]; }
@end
`,
}

func TestObjectiveCUnits(t *testing.T) {
	_, units := analyse(t, sdweb)
	m := units["SDWebImage#SDWebImageManager"]
	require.NotNil(t, m, "interface, class extension and implementation are one unit")
	assert.True(t, m.HasMarker("class"))
	assert.True(t, m.HasMarker("NSObject"))
	assert.True(t, m.HasMarker("SDImageLoader"))
	assert.True(t, m.HasMarker("NSCopying"))
	assert.True(t, units["SDWebImage#SDImageLoader"].HasMarker("interface"))
	assert.Contains(t, units, "SDWebImage#SDWebImageManager.sharedManager")
	assert.Equal(t, "SDWebImage#SDWebImageManager", units["SDWebImage#SDWebImageManager.load"].Owner)
	assert.NotContains(t, units, "SDWebImage#UIView", "a category of a platform class declares nothing")
	assert.Equal(t, "SDWebImage#UIView", units["SDWebImage#UIView.sd_setImage"].Owner)
}

func TestObjectiveCEdges(t *testing.T) {
	files, _ := analyse(t, sdweb)
	edges := func(p string) []string {
		var out []string
		for _, s := range files[p].Snippets {
			if s.Type == file.ComponentImport {
				out = append(out, s.Value)
			}
		}
		return out
	}
	assert.Equal(t, []string{"SDWebImage/Cache"}, edges("SDWebImage/Core/SDWebImageManager.h"), "an #import of a header in another folder")
	assert.Equal(t, []string{"SDWebImage/Cache"}, edges("SDWebImage/Core/SDWebImageManager.m"), "a class used in a message send")
	assert.Equal(t, []string{"SDWebImage/Core"}, edges("SDWebImage/Categories/UIView+WebCache.m"))
	var imports []string
	for _, s := range files["SDWebImage/Core/SDWebImageManager.h"].Snippets {
		if s.Type == file.ImportRaw {
			imports = append(imports, s.Value)
		}
	}
	assert.Equal(t, []string{"Foundation"}, imports)
}
