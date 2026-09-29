package objc

import (
	"sort"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/archstats/archstats/core/unit"
	"github.com/archstats/archstats/extensions/treesitter/apple"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A UIKit app in Objective-C, the mini-app the UI's iOS lanes are tested
// against for this language (archstats-ui:
// features/frameworks/layers.mobile.test.ts). No project file, so the target
// is the top-level folder.
var uikitApp = map[string]string{
	"App/AppDelegate.h": `#import <UIKit/UIKit.h>
@interface AppDelegate : UIResponder <UIApplicationDelegate>
@property (strong, nonatomic) UIWindow *window;
@end
`,
	"App/AppDelegate.m": `#import "AppDelegate.h"
#import "TimelineViewController.h"
@implementation AppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
  self.window.rootViewController = [[TimelineViewController alloc] init];
  return YES;
}
@end
`,
	"App/Timeline/TimelineViewController.h": `#import <UIKit/UIKit.h>
@interface TimelineViewController : UIViewController <UITableViewDataSource>
@end
`,
	"App/Timeline/TimelineViewController.m": `#import "TimelineViewController.h"
#import "StatusStore.h"
#import "StatusCell.h"

@interface TimelineViewController ()
@property (nonatomic, strong) StatusStore *store;
@end

@implementation TimelineViewController
- (void)viewDidLoad {
  [super viewDidLoad];
  [self.tableView registerClass:[StatusCell class] forCellReuseIdentifier:@"status"];
  [self.store loadWithCompletion:^(NSArray<Status *> *statuses) { [self.tableView reloadData]; }];
}
- (NSInteger)tableView:(UITableView *)tableView numberOfRowsInSection:(NSInteger)section { return self.store.statuses.count; }
@end
`,
	"App/Timeline/StatusCell.h": `#import <UIKit/UIKit.h>
@class Status;
@interface StatusCell : UITableViewCell
- (void)configureWithStatus:(Status *)status;
@end
`,
	"App/Timeline/StatusCell.m": `#import "StatusCell.h"
#import "Status.h"
@implementation StatusCell
- (void)configureWithStatus:(Status *)status { self.textLabel.text = status.content; }
@end
`,
	"App/State/StatusStore.h": `#import <Foundation/Foundation.h>
@class Status;
@interface StatusStore : NSObject
@property (nonatomic, readonly) NSArray<Status *> *statuses;
- (void)loadWithCompletion:(void (^)(NSArray<Status *> *))completion;
@end
`,
	"App/State/StatusStore.m": `#import "StatusStore.h"
#import "MastodonClient.h"
@implementation StatusStore
- (void)loadWithCompletion:(void (^)(NSArray<Status *> *))completion {
  [[MastodonClient shared] fetchTimeline:completion];
}
@end
`,
	"App/Network/MastodonClient.h": `#import <Foundation/Foundation.h>
@class Status;
@protocol MastodonClientDelegate <NSObject>
- (void)clientDidFail:(NSError *)error;
@end
@interface MastodonClient : NSObject
@property (nonatomic, weak) id<MastodonClientDelegate> delegate;
+ (instancetype)shared;
- (void)fetchTimeline:(void (^)(NSArray<Status *> *))completion;
@end
`,
	"App/Network/MastodonClient.m": `#import "MastodonClient.h"
#import "Status.h"
#import "TimelineViewController.h"
@implementation MastodonClient
+ (instancetype)shared { return nil; }
- (void)fetchTimeline:(void (^)(NSArray<Status *> *))completion {
  NSURLSessionDataTask *task = [[NSURLSession sharedSession] dataTaskWithURL:nil completionHandler:nil];
  [task resume];
}
// Planted: a client that knows a screen.
- (TimelineViewController *)screen { return [TimelineViewController new]; }
@end
`,
	"App/Models/Status.h": `#import <Foundation/Foundation.h>
@interface Status : NSObject <NSCoding, NSCopying>
@property (nonatomic, copy) NSString *identifier;
@property (nonatomic, copy) NSString *content;
@end
`,
	"App/Models/Status.m": `#import "Status.h"
#import "StatusStore.h"
@implementation Status
- (id)copyWithZone:(NSZone *)zone { return self; }
// Planted: a model that reaches back up to the store.
- (StatusStore *)store { return [StatusStore new]; }
@end
`,
	"App/Models/StatusEntity.h": `#import <CoreData/CoreData.h>
@interface StatusEntity : NSManagedObject
@property (nonatomic, copy) NSString *identifier;
@end
`,
	"AppTests/StatusStoreTests.m": `#import <XCTest/XCTest.h>
#import "StatusStore.h"
@interface StatusStoreTests : XCTestCase
@end
@implementation StatusStoreTests
- (void)testLoad { StatusStore *store = [StatusStore new]; XCTAssertNotNil(store); }
@end
`,
}

func markers(u *unit.Unit, source string) []string {
	var out []string
	for _, m := range u.Markers {
		if m.Source == source {
			out = append(out, m.Key)
		}
	}
	sort.Strings(out)
	return out
}

// rolledUp is what the UI reads for a unit: its own references and its
// members', since members are not listed apart from their owner.
func rolledUp(units map[string]*unit.Unit, id string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(u *unit.Unit) {
		for _, r := range u.Refs {
			key := r.Module + "#" + r.Name
			if !seen[key] && key != id {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	add(units[id])
	for _, u := range units {
		if u.Owner == id {
			add(u)
		}
	}
	sort.Strings(out)
	return out
}

func TestUIKitUnitsInObjectiveC(t *testing.T) {
	files, units := analyse(t, uikitApp)

	delegate := units["App#AppDelegate"]
	require.NotNil(t, delegate)
	assert.Equal(t, unit.KindType, delegate.Kind)
	assert.Equal(t, []string{"class"}, markers(delegate, apple.SourceKeyword))
	assert.Equal(t, []string{"UIApplicationDelegate", "UIResponder"}, markers(delegate, unit.SourceSupertype))

	screen := units["App#TimelineViewController"]
	require.NotNil(t, screen, "header, class extension and implementation are one unit")
	assert.Equal(t, []string{"UITableViewDataSource", "UIViewController"}, markers(screen, unit.SourceSupertype))
	assert.Equal(t, "App#TimelineViewController", units["App#TimelineViewController.viewDidLoad"].Owner)
	assert.Equal(t, []string{"UITableViewCell"}, markers(units["App#StatusCell"], unit.SourceSupertype))

	assert.Equal(t, []string{"NSObject"}, markers(units["App#StatusStore"], unit.SourceSupertype))
	assert.Equal(t, []string{"NSObject"}, markers(units["App#MastodonClient"], unit.SourceSupertype))
	assert.Equal(t, []string{"NSObject", "interface"}, markers(units["App#MastodonClientDelegate"], unit.SourceSupertype), "a protocol is an interface")
	assert.Equal(t, []string{"NSCoding", "NSCopying", "NSObject"}, markers(units["App#Status"], unit.SourceSupertype))
	assert.Equal(t, []string{"NSManagedObject"}, markers(units["App#StatusEntity"], unit.SourceSupertype))
	assert.Equal(t, []string{"XCTestCase"}, markers(units["AppTests#StatusStoreTests"], unit.SourceSupertype), "a test folder is a target of its own")

	// The imports detection reads: the framework, as Swift would name it.
	imports := func(p string) []string {
		var out []string
		for _, s := range files[p].Snippets {
			if s.Type == file.ImportRaw {
				out = append(out, s.Value)
			}
		}
		return out
	}
	assert.Equal(t, []string{"UIKit"}, imports("App/Timeline/TimelineViewController.h"))
	assert.Equal(t, []string{"Foundation"}, imports("App/Network/MastodonClient.h"))
	assert.Equal(t, []string{"CoreData"}, imports("App/Models/StatusEntity.h"))
	assert.Equal(t, []string{"XCTest"}, imports("AppTests/StatusStoreTests.m"))
	assert.Empty(t, imports("App/Timeline/TimelineViewController.m"), "an #import of the app's own header is not a framework")
}

func TestUIKitReferencesRunBetweenTheLanes(t *testing.T) {
	_, units := analyse(t, uikitApp)
	assert.Equal(t, []string{"App#TimelineViewController"}, rolledUp(units, "App#AppDelegate"))
	// Screen -> store and cell; the model only through the store's block type.
	assert.Equal(t, []string{"App#Status", "App#StatusCell", "App#StatusStore"}, rolledUp(units, "App#TimelineViewController"))
	assert.Equal(t, []string{"App#Status"}, rolledUp(units, "App#StatusCell"))
	assert.Equal(t, []string{"App#MastodonClient", "App#Status"}, rolledUp(units, "App#StatusStore"))
	// Client -> model, its delegate protocol, and the planted screen;
	// Foundation's own types resolve to nothing.
	assert.Equal(t, []string{"App#MastodonClientDelegate", "App#Status", "App#TimelineViewController"}, rolledUp(units, "App#MastodonClient"))
	// The planted model -> store reference.
	assert.Equal(t, []string{"App#StatusStore"}, rolledUp(units, "App#Status"))
	assert.Equal(t, []string{"App#StatusStore"}, rolledUp(units, "AppTests#StatusStoreTests"), "a test target's #import reaches the app's header")
}
