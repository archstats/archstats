package deployables

import (
	"reflect"
	"testing"
)

func TestReadDesktopConfigs(t *testing.T) {
	w := readWails("app/wails.json", []byte(`{"name":"archstats-desktop","outputfilename":"archstats-desktop","info":{"productName":"Archstats Desktop"}}`))
	if w == nil || w.dir != "app" || w.platform != "wails" || !reflect.DeepEqual(w.names, []string{"archstats-desktop", "archstats-desktop", "Archstats Desktop"}) {
		t.Errorf("wails = %+v", w)
	}
	tauri := readTauri("web/src-tauri/tauri.conf.json", []byte(`{"productName":"Notes","identifier":"com.x.notes"}`))
	if tauri == nil || tauri.dir != "web" || tauri.names[0] != "Notes" {
		t.Errorf("tauri = %+v", tauri)
	}
	if readWails("wails.json", []byte("{")) != nil {
		t.Error("malformed wails.json read")
	}
}
