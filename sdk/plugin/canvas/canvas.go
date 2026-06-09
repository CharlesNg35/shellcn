// Package canvas provides helpers for the plugin canvas stream protocol.
package canvas

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

var ErrInvalidInput = plugin.ErrInvalidInput

type PanelTheme = plugin.PanelTheme

const (
	PanelThemeLight = plugin.PanelThemeLight
	PanelThemeDark  = plugin.PanelThemeDark
)

type CommandType string

const (
	CommandClear          CommandType = "clear"
	CommandSet            CommandType = "set"
	CommandRegions        CommandType = "regions"
	CommandSave           CommandType = "save"
	CommandRestore        CommandType = "restore"
	CommandResetTransform CommandType = "resetTransform"
	CommandTranslate      CommandType = "translate"
	CommandScale          CommandType = "scale"
	CommandRotate         CommandType = "rotate"
	CommandTransform      CommandType = "transform"
	CommandStyle          CommandType = "style"
	CommandResetStyle     CommandType = "resetStyle"
	CommandLineDash       CommandType = "lineDash"
	CommandShadow         CommandType = "shadow"
	CommandGradient       CommandType = "gradient"
	CommandPattern        CommandType = "pattern"
	CommandClip           CommandType = "clip"
	CommandRect           CommandType = "rect"
	CommandLine           CommandType = "line"
	CommandPolyline       CommandType = "polyline"
	CommandPolygon        CommandType = "polygon"
	CommandArc            CommandType = "arc"
	CommandQuadraticCurve CommandType = "quadraticCurve"
	CommandBezierCurve    CommandType = "bezierCurve"
	CommandCircle         CommandType = "circle"
	CommandEllipse        CommandType = "ellipse"
	CommandPath           CommandType = "path"
	CommandText           CommandType = "text"
	CommandTextBox        CommandType = "textBox"
	CommandMeasureText    CommandType = "measureText"
	CommandImage          CommandType = "image"
	CommandImageData      CommandType = "imageData"
	CommandSnapshot       CommandType = "snapshot"
)

type EventType string

const (
	EventReady    EventType = "ready"
	EventResize   EventType = "resize"
	EventPointer  EventType = "pointer"
	EventWheel    EventType = "wheel"
	EventKey      EventType = "key"
	EventMetrics  EventType = "textMetrics"
	EventSnapshot EventType = "snapshot"
)

const (
	PointerDown   = "pointerdown"
	PointerMove   = "pointermove"
	PointerUp     = "pointerup"
	PointerCancel = "pointercancel"
)

type LineCap string

const (
	LineCapButt   LineCap = "butt"
	LineCapRound  LineCap = "round"
	LineCapSquare LineCap = "square"
)

type LineJoin string

const (
	LineJoinBevel LineJoin = "bevel"
	LineJoinRound LineJoin = "round"
	LineJoinMiter LineJoin = "miter"
)

type TextAlign string

const (
	TextAlignStart  TextAlign = "start"
	TextAlignEnd    TextAlign = "end"
	TextAlignLeft   TextAlign = "left"
	TextAlignRight  TextAlign = "right"
	TextAlignCenter TextAlign = "center"
)

type TextBaseline string

const (
	TextBaselineTop         TextBaseline = "top"
	TextBaselineHanging     TextBaseline = "hanging"
	TextBaselineMiddle      TextBaseline = "middle"
	TextBaselineAlphabetic  TextBaseline = "alphabetic"
	TextBaselineIdeographic TextBaseline = "ideographic"
	TextBaselineBottom      TextBaseline = "bottom"
)

type CompositeOperation string

const (
	CompositeSourceOver      CompositeOperation = "source-over"
	CompositeSourceIn        CompositeOperation = "source-in"
	CompositeSourceOut       CompositeOperation = "source-out"
	CompositeSourceAtop      CompositeOperation = "source-atop"
	CompositeDestinationOver CompositeOperation = "destination-over"
	CompositeDestinationIn   CompositeOperation = "destination-in"
	CompositeDestinationOut  CompositeOperation = "destination-out"
	CompositeDestinationAtop CompositeOperation = "destination-atop"
	CompositeLighter         CompositeOperation = "lighter"
	CompositeCopy            CompositeOperation = "copy"
	CompositeXOR             CompositeOperation = "xor"
	CompositeMultiply        CompositeOperation = "multiply"
	CompositeScreen          CompositeOperation = "screen"
	CompositeOverlay         CompositeOperation = "overlay"
	CompositeDarken          CompositeOperation = "darken"
	CompositeLighten         CompositeOperation = "lighten"
	CompositeColorDodge      CompositeOperation = "color-dodge"
	CompositeColorBurn       CompositeOperation = "color-burn"
	CompositeHardLight       CompositeOperation = "hard-light"
	CompositeSoftLight       CompositeOperation = "soft-light"
	CompositeDifference      CompositeOperation = "difference"
	CompositeExclusion       CompositeOperation = "exclusion"
	CompositeHue             CompositeOperation = "hue"
	CompositeSaturation      CompositeOperation = "saturation"
	CompositeColor           CompositeOperation = "color"
	CompositeLuminosity      CompositeOperation = "luminosity"
)

type Command interface {
	canvasCommand()
}

type Event interface {
	canvasEvent()
	EventType() EventType
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type RegionShape string

const (
	RegionRect    RegionShape = "rect"
	RegionCircle  RegionShape = "circle"
	RegionPolygon RegionShape = "polygon"
	RegionPath    RegionShape = "path"
)

type Region struct {
	ID             string      `json:"id"`
	Shape          RegionShape `json:"shape,omitempty"`
	X              float64     `json:"x"`
	Y              float64     `json:"y"`
	Width          float64     `json:"width,omitempty"`
	Height         float64     `json:"height,omitempty"`
	Radius         float64     `json:"radius,omitempty"`
	Points         []Point     `json:"points,omitempty"`
	D              string      `json:"d,omitempty"`
	Cursor         string      `json:"cursor,omitempty"`
	Label          string      `json:"label,omitempty"`
	CapturePointer bool        `json:"capturePointer,omitempty"`
}

type Frame struct {
	Commands []Command `json:"commands,omitempty"`
	Regions  []Region  `json:"regions,omitempty"`
}

type RawCommand struct {
	Type   CommandType    `json:"type"`
	Fields map[string]any `json:"-"`
}

func WriteFrame(w io.Writer, frame Frame) error {
	return json.NewEncoder(w).Encode(frame)
}

func DecodeEvent(r io.Reader) (Event, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	return ParseEvent(raw)
}

func ParseEvent(data []byte) (Event, error) {
	var header struct {
		Type EventType `json:"type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case EventReady:
		var ev ReadyEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventResize:
		var ev ResizeEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventPointer:
		var ev PointerEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventWheel:
		var ev WheelEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventKey:
		var ev KeyEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventMetrics:
		var ev TextMetricsEvent
		return &ev, json.Unmarshal(data, &ev)
	case EventSnapshot:
		var ev SnapshotEvent
		return &ev, json.Unmarshal(data, &ev)
	default:
		return nil, fmt.Errorf("%w: unknown canvas event type %q", ErrInvalidInput, header.Type)
	}
}

type Paint struct {
	Fill         string             `json:"fill,omitempty"`
	Stroke       string             `json:"stroke,omitempty"`
	LineWidth    float64            `json:"lineWidth,omitempty"`
	Font         string             `json:"font,omitempty"`
	Alpha        *float64           `json:"alpha,omitempty"`
	Composite    CompositeOperation `json:"composite,omitempty"`
	LineCap      LineCap            `json:"lineCap,omitempty"`
	LineJoin     LineJoin           `json:"lineJoin,omitempty"`
	TextAlign    TextAlign          `json:"textAlign,omitempty"`
	TextBaseline TextBaseline       `json:"textBaseline,omitempty"`
	FillID       string             `json:"fillId,omitempty"`
	StrokeID     string             `json:"strokeId,omitempty"`
	Filter       string             `json:"filter,omitempty"`
	Direction    string             `json:"direction,omitempty"`
	MiterLimit   *float64           `json:"miterLimit,omitempty"`

	NoFill   bool `json:"-"`
	NoStroke bool `json:"-"`
}

type Clear struct {
	Color string `json:"color,omitempty"`
}

type Set struct {
	Background string   `json:"background,omitempty"`
	Cursor     string   `json:"cursor,omitempty"`
	Regions    []Region `json:"regions,omitempty"`
}

type Regions struct {
	Items []Region `json:"items,omitempty"`
}

type (
	Save           struct{}
	Restore        struct{}
	ResetTransform struct{}
)

type Translate struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Scale struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Rotate struct {
	Angle float64 `json:"angle,omitempty"`
}

type Transform struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
	D float64 `json:"d"`
	E float64 `json:"e"`
	F float64 `json:"f"`
}

type Style struct {
	Paint
}

type ResetStyle struct{}

type LineDash struct {
	Segments []float64 `json:"segments,omitempty"`
	Offset   float64   `json:"offset,omitempty"`
}

type Shadow struct {
	Color   string  `json:"color,omitempty"`
	Blur    float64 `json:"blur,omitempty"`
	OffsetX float64 `json:"offsetX,omitempty"`
	OffsetY float64 `json:"offsetY,omitempty"`
}

type GradientKind string

const (
	GradientLinear GradientKind = "linear"
	GradientRadial GradientKind = "radial"
	GradientConic  GradientKind = "conic"
)

type GradientStop struct {
	Offset float64 `json:"offset"`
	Color  string  `json:"color"`
}

type Gradient struct {
	ID         string         `json:"id"`
	Kind       GradientKind   `json:"kind"`
	X0         float64        `json:"x0,omitempty"`
	Y0         float64        `json:"y0,omitempty"`
	X1         float64        `json:"x1,omitempty"`
	Y1         float64        `json:"y1,omitempty"`
	R0         float64        `json:"r0,omitempty"`
	R1         float64        `json:"r1,omitempty"`
	X          float64        `json:"x,omitempty"`
	Y          float64        `json:"y,omitempty"`
	StartAngle float64        `json:"startAngle,omitempty"`
	Stops      []GradientStop `json:"stops,omitempty"`
}

type Pattern struct {
	ID         string `json:"id"`
	Src        string `json:"src"`
	Repetition string `json:"repetition,omitempty"`
}

type Clip struct {
	Shape    RegionShape `json:"shape,omitempty"`
	D        string      `json:"d,omitempty"`
	X        float64     `json:"x,omitempty"`
	Y        float64     `json:"y,omitempty"`
	Width    float64     `json:"width,omitempty"`
	Height   float64     `json:"height,omitempty"`
	Radius   float64     `json:"radius,omitempty"`
	Points   []Point     `json:"points,omitempty"`
	FillRule string      `json:"fillRule,omitempty"`
}

type Rect struct {
	Paint
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

type Line struct {
	Paint
	X1 float64 `json:"x1,omitempty"`
	Y1 float64 `json:"y1,omitempty"`
	X2 float64 `json:"x2,omitempty"`
	Y2 float64 `json:"y2,omitempty"`
}

type Arc struct {
	Paint
	X                float64 `json:"x,omitempty"`
	Y                float64 `json:"y,omitempty"`
	Radius           float64 `json:"radius,omitempty"`
	StartAngle       float64 `json:"startAngle,omitempty"`
	EndAngle         float64 `json:"endAngle,omitempty"`
	CounterClockwise bool    `json:"counterclockwise,omitempty"`
}

type QuadraticCurve struct {
	Paint
	X0  float64 `json:"x0,omitempty"`
	Y0  float64 `json:"y0,omitempty"`
	CPX float64 `json:"cpx,omitempty"`
	CPY float64 `json:"cpy,omitempty"`
	X   float64 `json:"x,omitempty"`
	Y   float64 `json:"y,omitempty"`
}

type BezierCurve struct {
	Paint
	X0   float64 `json:"x0,omitempty"`
	Y0   float64 `json:"y0,omitempty"`
	CP1X float64 `json:"cp1x,omitempty"`
	CP1Y float64 `json:"cp1y,omitempty"`
	CP2X float64 `json:"cp2x,omitempty"`
	CP2Y float64 `json:"cp2y,omitempty"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
}

type Polyline struct {
	Paint
	Points []Point `json:"points,omitempty"`
}

type Polygon struct {
	Paint
	Points []Point `json:"points,omitempty"`
}

type Circle struct {
	Paint
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

type Ellipse struct {
	Paint
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	RadiusX  float64 `json:"radiusX,omitempty"`
	RadiusY  float64 `json:"radiusY,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
}

type Path struct {
	Paint
	D string `json:"d,omitempty"`
}

type Text struct {
	Paint
	X        float64  `json:"x,omitempty"`
	Y        float64  `json:"y,omitempty"`
	Text     string   `json:"text,omitempty"`
	MaxWidth *float64 `json:"maxWidth,omitempty"`
}

type TextBox struct {
	Paint
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Width      float64 `json:"width,omitempty"`
	LineHeight float64 `json:"lineHeight,omitempty"`
	Text       string  `json:"text,omitempty"`
}

type MeasureText struct {
	RequestID string `json:"requestId,omitempty"`
	Text      string `json:"text,omitempty"`
	Font      string `json:"font,omitempty"`
}

type Image struct {
	Paint
	Src          string  `json:"src,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	Width        float64 `json:"width,omitempty"`
	Height       float64 `json:"height,omitempty"`
	SourceX      float64 `json:"sourceX,omitempty"`
	SourceY      float64 `json:"sourceY,omitempty"`
	SourceWidth  float64 `json:"sourceWidth,omitempty"`
	SourceHeight float64 `json:"sourceHeight,omitempty"`
	Smoothing    *bool   `json:"smoothing,omitempty"`
}

type ImageData struct {
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
	Data   []int   `json:"data,omitempty"`
}

type Snapshot struct {
	RequestID     string  `json:"requestId,omitempty"`
	MIME          string  `json:"mime,omitempty"`
	Quality       float64 `json:"quality,omitempty"`
	MinIntervalMs int     `json:"minIntervalMs,omitempty"`
}

func (Clear) canvasCommand()          {}
func (RawCommand) canvasCommand()     {}
func (Set) canvasCommand()            {}
func (Regions) canvasCommand()        {}
func (Save) canvasCommand()           {}
func (Restore) canvasCommand()        {}
func (ResetTransform) canvasCommand() {}
func (Translate) canvasCommand()      {}
func (Scale) canvasCommand()          {}
func (Rotate) canvasCommand()         {}
func (Transform) canvasCommand()      {}
func (Style) canvasCommand()          {}
func (ResetStyle) canvasCommand()     {}
func (LineDash) canvasCommand()       {}
func (Shadow) canvasCommand()         {}
func (Gradient) canvasCommand()       {}
func (Pattern) canvasCommand()        {}
func (Clip) canvasCommand()           {}
func (Rect) canvasCommand()           {}
func (Line) canvasCommand()           {}
func (Arc) canvasCommand()            {}
func (QuadraticCurve) canvasCommand() {}
func (BezierCurve) canvasCommand()    {}
func (Polyline) canvasCommand()       {}
func (Polygon) canvasCommand()        {}
func (Circle) canvasCommand()         {}
func (Ellipse) canvasCommand()        {}
func (Path) canvasCommand()           {}
func (Text) canvasCommand()           {}
func (TextBox) canvasCommand()        {}
func (MeasureText) canvasCommand()    {}
func (Image) canvasCommand()          {}
func (ImageData) canvasCommand()      {}
func (Snapshot) canvasCommand()       {}

func (c Clear) MarshalJSON() ([]byte, error) {
	type payload Clear
	return marshalCommand(CommandClear, payload(c), nil)
}

func (c RawCommand) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(c.Fields)+1)
	for key, value := range c.Fields {
		out[key] = value
	}
	out["type"] = c.Type
	return json.Marshal(out)
}

func (c Set) MarshalJSON() ([]byte, error) {
	type payload Set
	return marshalCommand(CommandSet, payload(c), nil)
}

func (c Regions) MarshalJSON() ([]byte, error) {
	type payload Regions
	return marshalCommand(CommandRegions, payload(c), nil)
}

func (c Save) MarshalJSON() ([]byte, error) {
	type payload Save
	return marshalCommand(CommandSave, payload(c), nil)
}

func (c Restore) MarshalJSON() ([]byte, error) {
	type payload Restore
	return marshalCommand(CommandRestore, payload(c), nil)
}

func (c ResetTransform) MarshalJSON() ([]byte, error) {
	type payload ResetTransform
	return marshalCommand(CommandResetTransform, payload(c), nil)
}

func (c Translate) MarshalJSON() ([]byte, error) {
	type payload Translate
	return marshalCommand(CommandTranslate, payload(c), nil)
}

func (c Scale) MarshalJSON() ([]byte, error) {
	type payload Scale
	return marshalCommand(CommandScale, payload(c), nil)
}

func (c Rotate) MarshalJSON() ([]byte, error) {
	type payload Rotate
	return marshalCommand(CommandRotate, payload(c), nil)
}

func (c Transform) MarshalJSON() ([]byte, error) {
	type payload Transform
	return marshalCommand(CommandTransform, payload(c), nil)
}

func (c Style) MarshalJSON() ([]byte, error) {
	type payload Style
	return marshalCommand(CommandStyle, payload(c), &c.Paint)
}

func (c ResetStyle) MarshalJSON() ([]byte, error) {
	type payload ResetStyle
	return marshalCommand(CommandResetStyle, payload(c), nil)
}

func (c LineDash) MarshalJSON() ([]byte, error) {
	type payload LineDash
	return marshalCommand(CommandLineDash, payload(c), nil)
}

func (c Shadow) MarshalJSON() ([]byte, error) {
	type payload Shadow
	return marshalCommand(CommandShadow, payload(c), nil)
}

func (c Gradient) MarshalJSON() ([]byte, error) {
	type payload Gradient
	return marshalCommand(CommandGradient, payload(c), nil)
}

func (c Pattern) MarshalJSON() ([]byte, error) {
	type payload Pattern
	return marshalCommand(CommandPattern, payload(c), nil)
}

func (c Clip) MarshalJSON() ([]byte, error) {
	type payload Clip
	return marshalCommand(CommandClip, payload(c), nil)
}

func (c Rect) MarshalJSON() ([]byte, error) {
	type payload Rect
	return marshalCommand(CommandRect, payload(c), &c.Paint)
}

func (c Line) MarshalJSON() ([]byte, error) {
	type payload Line
	return marshalCommand(CommandLine, payload(c), &c.Paint)
}

func (c Arc) MarshalJSON() ([]byte, error) {
	type payload Arc
	return marshalCommand(CommandArc, payload(c), &c.Paint)
}

func (c QuadraticCurve) MarshalJSON() ([]byte, error) {
	type payload QuadraticCurve
	return marshalCommand(CommandQuadraticCurve, payload(c), &c.Paint)
}

func (c BezierCurve) MarshalJSON() ([]byte, error) {
	type payload BezierCurve
	return marshalCommand(CommandBezierCurve, payload(c), &c.Paint)
}

func (c Polyline) MarshalJSON() ([]byte, error) {
	type payload Polyline
	return marshalCommand(CommandPolyline, payload(c), &c.Paint)
}

func (c Polygon) MarshalJSON() ([]byte, error) {
	type payload Polygon
	return marshalCommand(CommandPolygon, payload(c), &c.Paint)
}

func (c Circle) MarshalJSON() ([]byte, error) {
	type payload Circle
	return marshalCommand(CommandCircle, payload(c), &c.Paint)
}

func (c Ellipse) MarshalJSON() ([]byte, error) {
	type payload Ellipse
	return marshalCommand(CommandEllipse, payload(c), &c.Paint)
}

func (c Path) MarshalJSON() ([]byte, error) {
	type payload Path
	return marshalCommand(CommandPath, payload(c), &c.Paint)
}

func (c Text) MarshalJSON() ([]byte, error) {
	type payload Text
	return marshalCommand(CommandText, payload(c), &c.Paint)
}

func (c TextBox) MarshalJSON() ([]byte, error) {
	type payload TextBox
	return marshalCommand(CommandTextBox, payload(c), &c.Paint)
}

func (c MeasureText) MarshalJSON() ([]byte, error) {
	type payload MeasureText
	return marshalCommand(CommandMeasureText, payload(c), nil)
}

func (c Image) MarshalJSON() ([]byte, error) {
	type payload Image
	return marshalCommand(CommandImage, payload(c), &c.Paint)
}

func (c ImageData) MarshalJSON() ([]byte, error) {
	type payload ImageData
	return marshalCommand(CommandImageData, payload(c), nil)
}

func (c Snapshot) MarshalJSON() ([]byte, error) {
	type payload Snapshot
	return marshalCommand(CommandSnapshot, payload(c), nil)
}

func marshalCommand(commandType CommandType, payload any, paint *Paint) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	out["type"] = commandType
	if paint != nil {
		paint.applyPaint(out)
	}
	return json.Marshal(out)
}

func (p Paint) applyPaint(out map[string]any) {
	if p.NoFill {
		out["fill"] = false
	}
	if p.NoStroke {
		out["stroke"] = false
	}
}

type ModifierState struct {
	Alt   bool `json:"alt,omitempty"`
	Ctrl  bool `json:"ctrl,omitempty"`
	Meta  bool `json:"meta,omitempty"`
	Shift bool `json:"shift,omitempty"`
}

type ReadyEvent struct {
	Type   EventType  `json:"type"`
	Width  float64    `json:"width"`
	Height float64    `json:"height"`
	DPR    float64    `json:"dpr"`
	Theme  PanelTheme `json:"theme,omitempty"`
}

func (ReadyEvent) canvasEvent() {}
func (ReadyEvent) EventType() EventType {
	return EventReady
}

type ResizeEvent struct {
	Type   EventType  `json:"type"`
	Width  float64    `json:"width"`
	Height float64    `json:"height"`
	DPR    float64    `json:"dpr"`
	Theme  PanelTheme `json:"theme,omitempty"`
}

func (ResizeEvent) canvasEvent() {}
func (ResizeEvent) EventType() EventType {
	return EventResize
}

type PointerEvent struct {
	Type        EventType     `json:"type"`
	Event       string        `json:"event"`
	X           float64       `json:"x"`
	Y           float64       `json:"y"`
	Button      int           `json:"button,omitempty"`
	Buttons     int           `json:"buttons,omitempty"`
	PointerID   int           `json:"pointerId,omitempty"`
	PointerType string        `json:"pointerType,omitempty"`
	RegionID    string        `json:"regionId,omitempty"`
	Modifiers   ModifierState `json:"modifiers,omitempty"`
}

func (PointerEvent) canvasEvent() {}
func (PointerEvent) EventType() EventType {
	return EventPointer
}

type WheelEvent struct {
	Type      EventType     `json:"type"`
	X         float64       `json:"x"`
	Y         float64       `json:"y"`
	DeltaX    float64       `json:"deltaX"`
	DeltaY    float64       `json:"deltaY"`
	DeltaMode int           `json:"deltaMode,omitempty"`
	Modifiers ModifierState `json:"modifiers,omitempty"`
}

func (WheelEvent) canvasEvent() {}
func (WheelEvent) EventType() EventType {
	return EventWheel
}

type KeyEvent struct {
	Type      EventType     `json:"type"`
	Event     string        `json:"event"`
	Key       string        `json:"key"`
	Code      string        `json:"code"`
	Repeat    bool          `json:"repeat,omitempty"`
	Modifiers ModifierState `json:"modifiers,omitempty"`
}

func (KeyEvent) canvasEvent() {}
func (KeyEvent) EventType() EventType {
	return EventKey
}

type TextMetricsEvent struct {
	Type                     EventType `json:"type"`
	RequestID                string    `json:"requestId,omitempty"`
	Text                     string    `json:"text,omitempty"`
	Width                    float64   `json:"width"`
	ActualBoundingBoxLeft    float64   `json:"actualBoundingBoxLeft,omitempty"`
	ActualBoundingBoxRight   float64   `json:"actualBoundingBoxRight,omitempty"`
	ActualBoundingBoxAscent  float64   `json:"actualBoundingBoxAscent,omitempty"`
	ActualBoundingBoxDescent float64   `json:"actualBoundingBoxDescent,omitempty"`
	FontBoundingBoxAscent    float64   `json:"fontBoundingBoxAscent,omitempty"`
	FontBoundingBoxDescent   float64   `json:"fontBoundingBoxDescent,omitempty"`
	EmHeightAscent           float64   `json:"emHeightAscent,omitempty"`
	EmHeightDescent          float64   `json:"emHeightDescent,omitempty"`
	HangingBaseline          float64   `json:"hangingBaseline,omitempty"`
	AlphabeticBaseline       float64   `json:"alphabeticBaseline,omitempty"`
	IdeographicBaseline      float64   `json:"ideographicBaseline,omitempty"`
}

func (TextMetricsEvent) canvasEvent() {}
func (TextMetricsEvent) EventType() EventType {
	return EventMetrics
}

type SnapshotEvent struct {
	Type      EventType `json:"type"`
	RequestID string    `json:"requestId,omitempty"`
	MIME      string    `json:"mime,omitempty"`
	DataURL   string    `json:"dataUrl,omitempty"`
	Width     float64   `json:"width,omitempty"`
	Height    float64   `json:"height,omitempty"`
}

func (SnapshotEvent) canvasEvent() {}
func (SnapshotEvent) EventType() EventType {
	return EventSnapshot
}
