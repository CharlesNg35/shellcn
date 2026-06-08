package plugin_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestCanvasFrameMarshalUsesTypedCommands(t *testing.T) {
	alpha := 0.5
	maxWidth := 240.0
	frame := plugin.CanvasFrame{
		Commands: []plugin.CanvasCommand{
			plugin.CanvasClear{Color: "#020617"},
			plugin.CanvasRect{
				CanvasPaint: plugin.CanvasPaint{
					Fill:      "#2563eb",
					Stroke:    "#93c5fd",
					LineWidth: 2,
				},
				X: 24, Y: 32, Width: 160, Height: 44, Radius: 8,
			},
			plugin.CanvasText{
				CanvasPaint: plugin.CanvasPaint{
					Fill:         "#ffffff",
					Font:         "600 16px Inter, sans-serif",
					Alpha:        &alpha,
					TextAlign:    plugin.CanvasTextAlignCenter,
					TextBaseline: plugin.CanvasTextBaselineMiddle,
				},
				X: 104, Y: 54, Text: "Open", MaxWidth: &maxWidth,
			},
			plugin.CanvasPath{
				CanvasPaint: plugin.CanvasPaint{Stroke: "#f97316", NoFill: true},
				D:           "M 0 0 L 10 10",
			},
		},
		Regions: []plugin.CanvasRegion{{
			ID: "open", X: 24, Y: 32, Width: 160, Height: 44, Cursor: "pointer",
		}},
	}

	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	commands := got["commands"].([]any)
	if commands[0].(map[string]any)["type"] != string(plugin.CanvasCommandClear) {
		t.Fatalf("clear command type missing: %s", data)
	}
	rect := commands[1].(map[string]any)
	if rect["type"] != string(plugin.CanvasCommandRect) || rect["radius"] != float64(8) {
		t.Fatalf("rect command encoded incorrectly: %#v", rect)
	}
	text := commands[2].(map[string]any)
	if text["alpha"] != 0.5 || text["textAlign"] != string(plugin.CanvasTextAlignCenter) {
		t.Fatalf("text style encoded incorrectly: %#v", text)
	}
	path := commands[3].(map[string]any)
	if path["fill"] != false || path["stroke"] != "#f97316" {
		t.Fatalf("path paint encoded incorrectly: %#v", path)
	}
	regions := got["regions"].([]any)
	if regions[0].(map[string]any)["id"] != "open" {
		t.Fatalf("region encoded incorrectly: %#v", regions[0])
	}
}

func TestWriteCanvasFrame(t *testing.T) {
	var out bytes.Buffer
	err := plugin.WriteCanvasFrame(&out, plugin.CanvasFrame{
		Commands: []plugin.CanvasCommand{plugin.CanvasCircle{X: 10, Y: 20, Radius: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"type":"circle"`)) {
		t.Fatalf("circle frame missing type: %s", out.String())
	}
}

func TestCanvasRawCommandMarshal(t *testing.T) {
	data, err := json.Marshal(plugin.CanvasFrame{
		Commands: []plugin.CanvasCommand{
			plugin.CanvasRawCommand{
				Type: plugin.CanvasCommandType("future"),
				Fields: map[string]any{
					"value": "custom",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"type":"future"`)) || !bytes.Contains(data, []byte(`"value":"custom"`)) {
		t.Fatalf("raw command encoded incorrectly: %s", data)
	}
}

func TestCanvasExpandedCommandsMarshal(t *testing.T) {
	quality := 0.82
	smoothing := false
	data, err := json.Marshal(plugin.CanvasFrame{Commands: []plugin.CanvasCommand{
		plugin.CanvasGradient{
			ID: "brand", Kind: plugin.CanvasGradientLinear, X0: 0, Y0: 0, X1: 100, Y1: 0,
			Stops: []plugin.CanvasGradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}},
		},
		plugin.CanvasLineDash{Segments: []float64{6, 3}, Offset: 2},
		plugin.CanvasShadow{Color: "#000", Blur: 10, OffsetX: 2, OffsetY: 4},
		plugin.CanvasClip{Shape: plugin.CanvasRegionCircle, X: 40, Y: 40, Radius: 20},
		plugin.CanvasArc{CanvasPaint: plugin.CanvasPaint{StrokeID: "brand", NoFill: true}, X: 40, Y: 40, Radius: 20, EndAngle: 3.14},
		plugin.CanvasQuadraticCurve{CanvasPaint: plugin.CanvasPaint{Stroke: "#fff", NoFill: true}, X0: 0, Y0: 0, CPX: 20, CPY: 50, X: 90, Y: 10},
		plugin.CanvasBezierCurve{CanvasPaint: plugin.CanvasPaint{Stroke: "#fff", NoFill: true}, X0: 0, Y0: 0, CP1X: 20, CP1Y: 50, CP2X: 70, CP2Y: 50, X: 90, Y: 10},
		plugin.CanvasTextBox{CanvasPaint: plugin.CanvasPaint{FillID: "brand"}, X: 10, Y: 10, Width: 180, Text: "wrapped text"},
		plugin.CanvasMeasureText{RequestID: "m1", Text: "measure", Font: "16px sans-serif"},
		plugin.CanvasImage{Src: "data:image/png;base64,", X: 0, Y: 0, SourceX: 2, SourceY: 3, SourceWidth: 10, SourceHeight: 11, Smoothing: &smoothing},
		plugin.CanvasImageData{X: 0, Y: 0, Width: 1, Height: 1, Data: []int{255, 0, 0, 255}},
		plugin.CanvasSnapshot{RequestID: "s1", MIME: "image/png", Quality: quality},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"type":"gradient"`,
		`"type":"lineDash"`,
		`"type":"shadow"`,
		`"type":"clip"`,
		`"type":"arc"`,
		`"type":"quadraticCurve"`,
		`"type":"bezierCurve"`,
		`"type":"textBox"`,
		`"type":"measureText"`,
		`"type":"imageData"`,
		`"type":"snapshot"`,
		`"strokeId":"brand"`,
		`"fillId":"brand"`,
		`"fill":false`,
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("expanded command frame missing %s:\n%s", want, data)
		}
	}
}

func TestParseCanvasEvent(t *testing.T) {
	readyEvent, err := plugin.ParseCanvasEvent([]byte(`{"type":"ready","width":800,"height":480,"dpr":2,"theme":"dark"}`))
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := readyEvent.(*plugin.CanvasReadyEvent)
	if !ok {
		t.Fatalf("got %T, want *plugin.CanvasReadyEvent", readyEvent)
	}
	if ready.Theme != plugin.PanelThemeDark || ready.Width != 800 || ready.DPR != 2 {
		t.Fatalf("decoded ready event incorrectly: %#v", ready)
	}

	ev, err := plugin.ParseCanvasEvent([]byte(`{
		"type":"pointer",
		"event":"pointerdown",
		"x":42,
		"y":64,
		"button":0,
		"buttons":1,
		"pointerId":7,
		"pointerType":"mouse",
		"regionId":"open",
		"modifiers":{"shift":true}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	pointer, ok := ev.(*plugin.CanvasPointerEvent)
	if !ok {
		t.Fatalf("got %T, want *plugin.CanvasPointerEvent", ev)
	}
	if pointer.EventType() != plugin.CanvasEventPointer || pointer.RegionID != "open" || !pointer.Modifiers.Shift {
		t.Fatalf("decoded pointer event incorrectly: %#v", pointer)
	}
}

func TestParseCanvasRendererEvents(t *testing.T) {
	metrics, err := plugin.ParseCanvasEvent([]byte(`{"type":"textMetrics","requestId":"m1","text":"abc","width":24}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev, ok := metrics.(*plugin.CanvasTextMetricsEvent); !ok || ev.RequestID != "m1" || ev.Width != 24 {
		t.Fatalf("metrics event decoded incorrectly: %#v", metrics)
	}

	snapshot, err := plugin.ParseCanvasEvent([]byte(`{"type":"snapshot","requestId":"s1","mime":"image/png","dataUrl":"data:image/png;base64,test","width":100,"height":50}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev, ok := snapshot.(*plugin.CanvasSnapshotEvent); !ok || ev.RequestID != "s1" || ev.Width != 100 {
		t.Fatalf("snapshot event decoded incorrectly: %#v", snapshot)
	}
}

func TestDecodeCanvasEventRejectsUnknownType(t *testing.T) {
	_, err := plugin.ParseCanvasEvent([]byte(`{"type":"unknown"}`))
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}
