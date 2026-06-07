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

func TestParseCanvasEvent(t *testing.T) {
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

func TestDecodeCanvasEventRejectsUnknownType(t *testing.T) {
	_, err := plugin.ParseCanvasEvent([]byte(`{"type":"unknown"}`))
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}
