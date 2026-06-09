package canvas_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin/canvas"
)

func TestFrameMarshalUsesTypedCommands(t *testing.T) {
	alpha := 0.5
	maxWidth := 240.0
	frame := canvas.Frame{
		Commands: []canvas.Command{
			canvas.Clear{Color: "#020617", X: 4, Y: 8, Width: 320, Height: 180},
			canvas.Rect{
				Paint: canvas.Paint{
					Fill:      "#2563eb",
					Stroke:    "#93c5fd",
					LineWidth: 2,
				},
				X: 24, Y: 32, Width: 160, Height: 44, Radii: &canvas.Radii{
					TopLeft: 12, TopRight: 16, BottomRight: 8, BottomLeft: 4,
				},
			},
			canvas.Text{
				Paint: canvas.Paint{
					Fill:         "#ffffff",
					Font:         "600 16px Inter, sans-serif",
					Alpha:        &alpha,
					TextAlign:    canvas.TextAlignCenter,
					TextBaseline: canvas.TextBaselineMiddle,
				},
				X: 104, Y: 54, Text: "Open", MaxWidth: &maxWidth,
			},
			canvas.Path{
				Paint: canvas.Paint{Stroke: "#f97316", NoFill: true},
				D:     "M 0 0 L 10 10", FillRule: canvas.FillRuleEvenOdd,
			},
			canvas.FillText{Paint: canvas.Paint{Fill: "#fff"}, X: 8, Y: 8, Text: "Fill only"},
			canvas.StrokeText{Paint: canvas.Paint{Stroke: "#fff"}, X: 8, Y: 28, Text: "Stroke only"},
			canvas.Cursor{Value: "crosshair"},
			canvas.FocusRegion{ID: "open"},
			canvas.Announce{Text: "Open focused", Mode: canvas.AnnouncePolite},
			canvas.Snapshot{RequestID: "preview", MinIntervalMs: 250},
		},
		Regions: []canvas.Region{
			canvas.RectRegion("open", 24, 32, 160, 44, canvas.WithCursor("pointer"), canvas.WithLabel("Open")),
		},
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
	if commands[0].(map[string]any)["type"] != string(canvas.CommandClear) {
		t.Fatalf("clear command type missing: %s", data)
	}
	if commands[0].(map[string]any)["width"] != float64(320) {
		t.Fatalf("clear rect encoded incorrectly: %#v", commands[0])
	}
	rect := commands[1].(map[string]any)
	if rect["type"] != string(canvas.CommandRect) {
		t.Fatalf("rect command encoded incorrectly: %#v", rect)
	}
	radii := rect["radii"].(map[string]any)
	if radii["topRight"] != float64(16) || radii["bottomLeft"] != float64(4) {
		t.Fatalf("rect radii encoded incorrectly: %#v", rect)
	}
	text := commands[2].(map[string]any)
	if text["alpha"] != 0.5 || text["textAlign"] != string(canvas.TextAlignCenter) {
		t.Fatalf("text style encoded incorrectly: %#v", text)
	}
	path := commands[3].(map[string]any)
	if path["fill"] != false || path["stroke"] != "#f97316" || path["fillRule"] != string(canvas.FillRuleEvenOdd) {
		t.Fatalf("path paint encoded incorrectly: %#v", path)
	}
	if commands[4].(map[string]any)["type"] != string(canvas.CommandFillText) {
		t.Fatalf("fillText command encoded incorrectly: %#v", commands[4])
	}
	if commands[5].(map[string]any)["type"] != string(canvas.CommandStrokeText) {
		t.Fatalf("strokeText command encoded incorrectly: %#v", commands[5])
	}
	if commands[6].(map[string]any)["type"] != string(canvas.CommandCursor) {
		t.Fatalf("cursor command encoded incorrectly: %#v", commands[6])
	}
	if commands[7].(map[string]any)["type"] != string(canvas.CommandFocusRegion) {
		t.Fatalf("focusRegion command encoded incorrectly: %#v", commands[7])
	}
	if commands[8].(map[string]any)["type"] != string(canvas.CommandAnnounce) {
		t.Fatalf("announce command encoded incorrectly: %#v", commands[8])
	}
	snapshot := commands[9].(map[string]any)
	if snapshot["type"] != string(canvas.CommandSnapshot) || snapshot["minIntervalMs"] != float64(250) {
		t.Fatalf("snapshot encoded incorrectly: %#v", snapshot)
	}
	regions := got["regions"].([]any)
	if regions[0].(map[string]any)["id"] != "open" || regions[0].(map[string]any)["label"] != "Open" {
		t.Fatalf("region encoded incorrectly: %#v", regions[0])
	}
}

func TestWriteFrame(t *testing.T) {
	var out bytes.Buffer
	err := canvas.WriteFrame(&out, canvas.Frame{
		Commands: []canvas.Command{canvas.Circle{X: 10, Y: 20, Radius: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"type":"circle"`)) {
		t.Fatalf("circle frame missing type: %s", out.String())
	}
}

func TestRawCommandMarshal(t *testing.T) {
	data, err := json.Marshal(canvas.Frame{
		Commands: []canvas.Command{
			canvas.RawCommand{
				Type: canvas.CommandType("future"),
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

func TestExpandedCommandsMarshal(t *testing.T) {
	quality := 0.82
	smoothing := false
	data, err := json.Marshal(canvas.Frame{Commands: []canvas.Command{
		canvas.Gradient{
			ID: "brand", Kind: canvas.GradientLinear, X0: 0, Y0: 0, X1: 100, Y1: 0,
			Stops: []canvas.GradientStop{{Offset: 0, Color: "#000"}, {Offset: 1, Color: "#fff"}},
		},
		canvas.LineDash{Segments: []float64{6, 3}, Offset: 2},
		canvas.Shadow{Color: "#000", Blur: 10, OffsetX: 2, OffsetY: 4},
		canvas.Clip{Shape: canvas.RegionCircle, X: 40, Y: 40, Radius: 20},
		canvas.Arc{Paint: canvas.Paint{StrokeID: "brand", NoFill: true}, X: 40, Y: 40, Radius: 20, EndAngle: 3.14},
		canvas.QuadraticCurve{Paint: canvas.Paint{Stroke: "#fff", NoFill: true}, X0: 0, Y0: 0, CPX: 20, CPY: 50, X: 90, Y: 10},
		canvas.BezierCurve{Paint: canvas.Paint{Stroke: "#fff", NoFill: true}, X0: 0, Y0: 0, CP1X: 20, CP1Y: 50, CP2X: 70, CP2Y: 50, X: 90, Y: 10},
		canvas.TextBox{
			Paint: canvas.Paint{FillID: "brand"},
			X:     10, Y: 10, Width: 180, Height: 72, Padding: 10, MaxLines: 2,
			Ellipsis: "...", VerticalAlign: canvas.TextVerticalAlignMiddle,
			Background: "#020617", Radius: 10, Text: "wrapped text",
		},
		canvas.MeasureText{RequestID: "m1", Text: "measure", Font: "16px sans-serif"},
		canvas.Image{Src: "data:image/png;base64,", X: 0, Y: 0, SourceX: 2, SourceY: 3, SourceWidth: 10, SourceHeight: 11, Smoothing: &smoothing},
		canvas.ImageData{X: 0, Y: 0, Width: 1, Height: 1, Data: []int{255, 0, 0, 255}},
		canvas.Snapshot{RequestID: "s1", MIME: "image/png", Quality: quality},
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
		`"maxLines":2`,
		`"verticalAlign":"middle"`,
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

func TestRegionHelpers(t *testing.T) {
	rect := canvas.RectRegion(
		"button",
		1, 2, 3, 4,
		canvas.WithCursor("pointer"),
		canvas.WithLabel("Button"),
		canvas.WithCapturePointer(),
		canvas.WithRadii(canvas.Radii{TopLeft: 6, BottomRight: 8}),
	)
	if rect.Shape != canvas.RegionRect || rect.Cursor != "pointer" || !rect.CapturePointer {
		t.Fatalf("rect region helper returned unexpected region: %#v", rect)
	}
	if rect.Radii == nil || rect.Radii.BottomRight != 8 {
		t.Fatalf("rect region helper did not apply radii: %#v", rect)
	}

	circle := canvas.CircleRegion("node", 10, 20, 5, canvas.WithLabel("Node"))
	if circle.Shape != canvas.RegionCircle || circle.Label != "Node" || circle.Radius != 5 {
		t.Fatalf("circle region helper returned unexpected region: %#v", circle)
	}
}

func TestParseEvent(t *testing.T) {
	readyEvent, err := canvas.ParseEvent([]byte(`{"type":"ready","width":800,"height":480,"dpr":2,"theme":"dark"}`))
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := readyEvent.(*canvas.ReadyEvent)
	if !ok {
		t.Fatalf("got %T, want *canvas.ReadyEvent", readyEvent)
	}
	if ready.Theme != canvas.PanelThemeDark || ready.Width != 800 || ready.DPR != 2 {
		t.Fatalf("decoded ready event incorrectly: %#v", ready)
	}

	ev, err := canvas.ParseEvent([]byte(`{
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
	pointer, ok := ev.(*canvas.PointerEvent)
	if !ok {
		t.Fatalf("got %T, want *canvas.PointerEvent", ev)
	}
	if pointer.EventType() != canvas.EventPointer || pointer.RegionID != "open" || !pointer.Modifiers.Shift {
		t.Fatalf("decoded pointer event incorrectly: %#v", pointer)
	}
}

func TestParseRendererEvents(t *testing.T) {
	metrics, err := canvas.ParseEvent([]byte(`{"type":"textMetrics","requestId":"m1","text":"abc","width":24}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev, ok := metrics.(*canvas.TextMetricsEvent); !ok || ev.RequestID != "m1" || ev.Width != 24 {
		t.Fatalf("metrics event decoded incorrectly: %#v", metrics)
	}

	snapshot, err := canvas.ParseEvent([]byte(`{"type":"snapshot","requestId":"s1","mime":"image/png","dataUrl":"data:image/png;base64,test","width":100,"height":50}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev, ok := snapshot.(*canvas.SnapshotEvent); !ok || ev.RequestID != "s1" || ev.Width != 100 {
		t.Fatalf("snapshot event decoded incorrectly: %#v", snapshot)
	}
}

func TestDecodeEventRejectsUnknownType(t *testing.T) {
	_, err := canvas.ParseEvent([]byte(`{"type":"unknown"}`))
	if !errors.Is(err, canvas.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}
