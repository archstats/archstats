package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const manifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.POST_NOTIFICATIONS" />
    <uses-feature android:name="android.hardware.camera" android:required="false" />
    <application android:name=".NiaApplication">
        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
            <intent-filter android:autoVerify="true">
                <action android:name="android.intent.action.VIEW" />
                <category android:name="android.intent.category.DEFAULT" />
                <category android:name="android.intent.category.BROWSABLE" />
                <data android:scheme="https" />
                <data android:host="www.nowinandroid.apps.samples.google.com" />
                <data android:pathPrefix="/foryou" />
            </intent-filter>
        </activity>
        <service android:name="com.acme.sync.SyncService" android:exported="false" />
        <receiver android:name="Boot">
            <intent-filter><action android:name="android.intent.action.BOOT_COMPLETED" /></intent-filter>
        </receiver>
    </application>
</manifest>`

func TestAndroidManifest(t *testing.T) {
	m := readAndroidManifest([]byte(manifest))
	require.NotNil(t, m)
	byKind := map[string][]manifestEntry{}
	for _, e := range m.Entries {
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	require.Len(t, byKind["permission"], 2)
	assert.Equal(t, "android.permission.INTERNET", byKind["permission"][0].Name)
	assert.Equal(t, 3, byKind["permission"][0].Line)
	assert.Equal(t, "false", byKind["feature"][0].Value, "an optional feature says so")

	require.Len(t, byKind["activity"], 1)
	main := byKind["activity"][0]
	assert.Equal(t, ".MainActivity", main.Name)
	assert.True(t, main.Launcher)
	assert.Equal(t, "true", main.Exported)
	assert.Equal(t, 7, main.Line)

	require.Len(t, byKind["deep_link"], 1, "a filter's data elements combine into one link")
	assert.Equal(t, "https://www.nowinandroid.apps.samples.google.com/foryou", byKind["deep_link"][0].Value)
	assert.Equal(t, ".MainActivity", byKind["deep_link"][0].Name)

	assert.Equal(t, "false", byKind["service"][0].Exported)
	assert.Equal(t, "implied", byKind["receiver"][0].Exported, "an intent filter exported a component unless it said otherwise")
	assert.Equal(t, ".NiaApplication", byKind["application"][0].Name)
}

func TestClassName(t *testing.T) {
	assert.Equal(t, "com.acme.MainActivity", className(".MainActivity", "com.acme"))
	assert.Equal(t, "com.acme.Boot", className("Boot", "com.acme"))
	assert.Equal(t, "org.x.Sync", className("org.x.Sync", "com.acme"))
	assert.Equal(t, "com.acme.Outer.Inner", className(".Outer$Inner", "com.acme"))
}

const info = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>$(PRODUCT_BUNDLE_IDENTIFIER)</string>
	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLSchemes</key>
			<array><string>icecubesapp</string></array>
		</dict>
	</array>
	<key>NSCameraUsageDescription</key>
	<string>To take pictures</string>
	<key>UIBackgroundModes</key>
	<array><string>fetch</string><string>remote-notification</string></array>
	<key>NSAppTransportSecurity</key>
	<dict><key>NSAllowsArbitraryLoads</key><true/></dict>
	<key>UIApplicationSceneManifest</key>
	<dict>
		<key>UISceneConfigurations</key>
		<dict>
			<key>UIWindowSceneSessionRoleApplication</key>
			<array><dict><key>UISceneDelegateClassName</key><string>$(PRODUCT_MODULE_NAME).SceneDelegate</string></dict></array>
		</dict>
	</dict>
</dict>
</plist>`

func TestInfoPlist(t *testing.T) {
	entries := infoPlistEntries(readPlist([]byte(info)))
	got := map[string][]string{}
	for _, e := range entries {
		got[e.Kind] = append(got[e.Kind], e.Name+"="+e.Value)
	}
	assert.Equal(t, []string{"icecubesapp="}, got["url_scheme"])
	assert.Equal(t, []string{"NSCameraUsageDescription=To take pictures"}, got["usage_description"])
	assert.Equal(t, []string{"fetch=", "remote-notification="}, got["background_mode"])
	assert.Equal(t, []string{"NSAllowsArbitraryLoads=true"}, got["ats_exception"])
	assert.Equal(t, []string{"$(PRODUCT_MODULE_NAME).SceneDelegate="}, got["scene_delegate"])
	assert.Equal(t, []string{"$(PRODUCT_BUNDLE_IDENTIFIER)="}, got["bundle_id"])
}

func TestEntitlements(t *testing.T) {
	ent := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
	<key>com.apple.developer.associated-domains</key>
	<array><string>applinks:mastodon.social</string><string>webcredentials:x.org</string></array>
	<key>aps-environment</key><string>development</string>
</dict></plist>`
	entries := entitlementEntries(readPlist([]byte(ent)))
	var links []string
	kinds := map[string]int{}
	for _, e := range entries {
		kinds[e.Kind]++
		if e.Kind == "deep_link" {
			links = append(links, e.Value)
		}
	}
	assert.Equal(t, 2, kinds["entitlement"])
	assert.Equal(t, []string{"https://mastodon.social"}, links)
}
