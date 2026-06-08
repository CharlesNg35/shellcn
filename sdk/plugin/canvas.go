package plugin

import (
	"encoding/json"
	"fmt"
	"io"
)

type CanvasCommandType string

const (
	CanvasCommandClear          CanvasCommandType = "clear"
	CanvasCommandSet            CanvasCommandType = "set"
	CanvasCommandRegions        CanvasCommandType = "regions"
	CanvasCommandSave           CanvasCommandType = "save"
	CanvasCommandRestore        CanvasCommandType = "restore"
	CanvasCommandResetTransform CanvasCommandType = "resetTransform"
	CanvasCommandTranslate      CanvasCommandType = "translate"
	CanvasCommandScale          CanvasCommandType = "scale"
	CanvasCommandRotate         CanvasCommandType = "rotate"
	CanvasCommandTransform      CanvasCommandType = "transform"
	CanvasCommandStyle          CanvasCommandType = "style"
	CanvasCommandResetStyle     CanvasCommandType = "resetStyle"
	CanvasCommandLineDash       CanvasCommandType = "lineDash"
	CanvasCommandShadow         CanvasCommandType = "shadow"
	CanvasCommandGradient       CanvasCommandType = "gradient"
	CanvasCommandPattern        CanvasCommandType = "pattern"
	CanvasCommandClip           CanvasCommandType = "clip"
	CanvasCommandRect           CanvasCommandType = "rect"
	CanvasCommandLine           CanvasCommandType = "line"
	CanvasCommandPolyline       CanvasCommandType = "polyline"
	CanvasCommandPolygon        CanvasCommandType = "polygon"
	CanvasCommandArc            CanvasCommandType = "arc"
	CanvasCommandQuadraticCurve CanvasCommandType = "quadraticCurve"
	CanvasCommandBezierCurve    CanvasCommandType = "bezierCurve"
	CanvasCommandCircle         CanvasCommandType = "circle"
	CanvasCommandEllipse        CanvasCommandType = "ellipse"
	CanvasCommandPath           CanvasCommandType = "path"
	CanvasCommandText           CanvasCommandType = "text"
	CanvasCommandTextBox        CanvasCommandType = "textBox"
	CanvasCommandMeasureText    CanvasCommandType = "measureText"
	CanvasCommandImage          CanvasCommandType = "image"
	CanvasCommandImageData      CanvasCommandType = "imageData"
	CanvasCommandSnapshot       CanvasCommandType = "snapshot"
)

type CanvasEventType string

const (
	CanvasEventReady    CanvasEventType = "ready"
	CanvasEventResize   CanvasEventType = "resize"
	CanvasEventPointer  CanvasEventType = "pointer"
	CanvasEventWheel    CanvasEventType = "wheel"
	CanvasEventKey      CanvasEventType = "key"
	CanvasEventMetrics  CanvasEventType = "textMetrics"
	CanvasEventSnapshot CanvasEventType = "snapshot"
)

const (
	CanvasPointerDown   = "pointerdown"
	CanvasPointerMove   = "pointermove"
	CanvasPointerUp     = "pointerup"
	CanvasPointerCancel = "pointercancel"
)

type CanvasLineCap string

const (
	CanvasLineCapButt   CanvasLineCap = "butt"
	CanvasLineCapRound  CanvasLineCap = "round"
	CanvasLineCapSquare CanvasLineCap = "square"
)

type CanvasLineJoin string

const (
	CanvasLineJoinBevel CanvasLineJoin = "bevel"
	CanvasLineJoinRound CanvasLineJoin = "round"
	CanvasLineJoinMiter CanvasLineJoin = "miter"
)

type CanvasTextAlign string

const (
	CanvasTextAlignStart  CanvasTextAlign = "start"
	CanvasTextAlignEnd    CanvasTextAlign = "end"
	CanvasTextAlignLeft   CanvasTextAlign = "left"
	CanvasTextAlignRight  CanvasTextAlign = "right"
	CanvasTextAlignCenter CanvasTextAlign = "center"
)

type CanvasTextBaseline string

const (
	CanvasTextBaselineTop         CanvasTextBaseline = "top"
	CanvasTextBaselineHanging     CanvasTextBaseline = "hanging"
	CanvasTextBaselineMiddle      CanvasTextBaseline = "middle"
	CanvasTextBaselineAlphabetic  CanvasTextBaseline = "alphabetic"
	CanvasTextBaselineIdeographic CanvasTextBaseline = "ideographic"
	CanvasTextBaselineBottom      CanvasTextBaseline = "bottom"
)

type CanvasCompositeOperation string

const (
	CanvasCompositeSourceOver      CanvasCompositeOperation = "source-over"
	CanvasCompositeSourceIn        CanvasCompositeOperation = "source-in"
	CanvasCompositeSourceOut       CanvasCompositeOperation = "source-out"
	CanvasCompositeSourceAtop      CanvasCompositeOperation = "source-atop"
	CanvasCompositeDestinationOver CanvasCompositeOperation = "destination-over"
	CanvasCompositeDestinationIn   CanvasCompositeOperation = "destination-in"
	CanvasCompositeDestinationOut  CanvasCompositeOperation = "destination-out"
	CanvasCompositeDestinationAtop CanvasCompositeOperation = "destination-atop"
	CanvasCompositeLighter         CanvasCompositeOperation = "lighter"
	CanvasCompositeCopy            CanvasCompositeOperation = "copy"
	CanvasCompositeXOR             CanvasCompositeOperation = "xor"
	CanvasCompositeMultiply        CanvasCompositeOperation = "multiply"
	CanvasCompositeScreen          CanvasCompositeOperation = "screen"
	CanvasCompositeOverlay         CanvasCompositeOperation = "overlay"
	CanvasCompositeDarken          CanvasCompositeOperation = "darken"
	CanvasCompositeLighten         CanvasCompositeOperation = "lighten"
	CanvasCompositeColorDodge      CanvasCompositeOperation = "color-dodge"
	CanvasCompositeColorBurn       CanvasCompositeOperation = "color-burn"
	CanvasCompositeHardLight       CanvasCompositeOperation = "hard-light"
	CanvasCompositeSoftLight       CanvasCompositeOperation = "soft-light"
	CanvasCompositeDifference      CanvasCompositeOperation = "difference"
	CanvasCompositeExclusion       CanvasCompositeOperation = "exclusion"
	CanvasCompositeHue             CanvasCompositeOperation = "hue"
	CanvasCompositeSaturation      CanvasCompositeOperation = "saturation"
	CanvasCompositeColor           CanvasCompositeOperation = "color"
	CanvasCompositeLuminosity      CanvasCompositeOperation = "luminosity"
)

type CanvasCommand interface {
	canvasCommand()
}

type CanvasEvent interface {
	canvasEvent()
	EventType() CanvasEventType
}

type CanvasPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CanvasRegionShape string

const (
	CanvasRegionRect    CanvasRegionShape = "rect"
	CanvasRegionCircle  CanvasRegionShape = "circle"
	CanvasRegionPolygon CanvasRegionShape = "polygon"
	CanvasRegionPath    CanvasRegionShape = "path"
)

type CanvasRegion struct {
	ID             string            `json:"id"`
	Shape          CanvasRegionShape `json:"shape,omitempty"`
	X              float64           `json:"x"`
	Y              float64           `json:"y"`
	Width          float64           `json:"width,omitempty"`
	Height         float64           `json:"height,omitempty"`
	Radius         float64           `json:"radius,omitempty"`
	Points         []CanvasPoint     `json:"points,omitempty"`
	D              string            `json:"d,omitempty"`
	Cursor         string            `json:"cursor,omitempty"`
	Label          string            `json:"label,omitempty"`
	CapturePointer bool              `json:"capturePointer,omitempty"`
}

type CanvasFrame struct {
	Commands []CanvasCommand `json:"commands,omitempty"`
	Regions  []CanvasRegion  `json:"regions,omitempty"`
}

type CanvasRawCommand struct {
	Type   CanvasCommandType `json:"type"`
	Fields map[string]any    `json:"-"`
}

func WriteCanvasFrame(w io.Writer, frame CanvasFrame) error {
	return json.NewEncoder(w).Encode(frame)
}

func DecodeCanvasEvent(r io.Reader) (CanvasEvent, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}
	return ParseCanvasEvent(raw)
}

func ParseCanvasEvent(data []byte) (CanvasEvent, error) {
	var header struct {
		Type CanvasEventType `json:"type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	switch header.Type {
	case CanvasEventReady:
		var ev CanvasReadyEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventResize:
		var ev CanvasResizeEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventPointer:
		var ev CanvasPointerEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventWheel:
		var ev CanvasWheelEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventKey:
		var ev CanvasKeyEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventMetrics:
		var ev CanvasTextMetricsEvent
		return &ev, json.Unmarshal(data, &ev)
	case CanvasEventSnapshot:
		var ev CanvasSnapshotEvent
		return &ev, json.Unmarshal(data, &ev)
	default:
		return nil, fmt.Errorf("%w: unknown canvas event type %q", ErrInvalidInput, header.Type)
	}
}

type CanvasPaint struct {
	Fill         string                   `json:"fill,omitempty"`
	Stroke       string                   `json:"stroke,omitempty"`
	LineWidth    float64                  `json:"lineWidth,omitempty"`
	Font         string                   `json:"font,omitempty"`
	Alpha        *float64                 `json:"alpha,omitempty"`
	Composite    CanvasCompositeOperation `json:"composite,omitempty"`
	LineCap      CanvasLineCap            `json:"lineCap,omitempty"`
	LineJoin     CanvasLineJoin           `json:"lineJoin,omitempty"`
	TextAlign    CanvasTextAlign          `json:"textAlign,omitempty"`
	TextBaseline CanvasTextBaseline       `json:"textBaseline,omitempty"`
	FillID       string                   `json:"fillId,omitempty"`
	StrokeID     string                   `json:"strokeId,omitempty"`
	Filter       string                   `json:"filter,omitempty"`
	Direction    string                   `json:"direction,omitempty"`
	MiterLimit   *float64                 `json:"miterLimit,omitempty"`

	NoFill   bool `json:"-"`
	NoStroke bool `json:"-"`
}

type CanvasClear struct {
	Color string `json:"color,omitempty"`
}

type CanvasSet struct {
	Background string         `json:"background,omitempty"`
	Cursor     string         `json:"cursor,omitempty"`
	Regions    []CanvasRegion `json:"regions,omitempty"`
}

type CanvasRegions struct {
	Items []CanvasRegion `json:"items,omitempty"`
}

type (
	CanvasSave           struct{}
	CanvasRestore        struct{}
	CanvasResetTransform struct{}
)

type CanvasTranslate struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type CanvasScale struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type CanvasRotate struct {
	Angle float64 `json:"angle,omitempty"`
}

type CanvasTransform struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
	D float64 `json:"d"`
	E float64 `json:"e"`
	F float64 `json:"f"`
}

type CanvasStyle struct {
	CanvasPaint
}

type CanvasResetStyle struct{}

type CanvasLineDash struct {
	Segments []float64 `json:"segments,omitempty"`
	Offset   float64   `json:"offset,omitempty"`
}

type CanvasShadow struct {
	Color   string  `json:"color,omitempty"`
	Blur    float64 `json:"blur,omitempty"`
	OffsetX float64 `json:"offsetX,omitempty"`
	OffsetY float64 `json:"offsetY,omitempty"`
}

type CanvasGradientKind string

const (
	CanvasGradientLinear CanvasGradientKind = "linear"
	CanvasGradientRadial CanvasGradientKind = "radial"
	CanvasGradientConic  CanvasGradientKind = "conic"
)

type CanvasGradientStop struct {
	Offset float64 `json:"offset"`
	Color  string  `json:"color"`
}

type CanvasGradient struct {
	ID         string               `json:"id"`
	Kind       CanvasGradientKind   `json:"kind"`
	X0         float64              `json:"x0,omitempty"`
	Y0         float64              `json:"y0,omitempty"`
	X1         float64              `json:"x1,omitempty"`
	Y1         float64              `json:"y1,omitempty"`
	R0         float64              `json:"r0,omitempty"`
	R1         float64              `json:"r1,omitempty"`
	X          float64              `json:"x,omitempty"`
	Y          float64              `json:"y,omitempty"`
	StartAngle float64              `json:"startAngle,omitempty"`
	Stops      []CanvasGradientStop `json:"stops,omitempty"`
}

type CanvasPattern struct {
	ID         string `json:"id"`
	Src        string `json:"src"`
	Repetition string `json:"repetition,omitempty"`
}

type CanvasClip struct {
	Shape    CanvasRegionShape `json:"shape,omitempty"`
	D        string            `json:"d,omitempty"`
	X        float64           `json:"x,omitempty"`
	Y        float64           `json:"y,omitempty"`
	Width    float64           `json:"width,omitempty"`
	Height   float64           `json:"height,omitempty"`
	Radius   float64           `json:"radius,omitempty"`
	Points   []CanvasPoint     `json:"points,omitempty"`
	FillRule string            `json:"fillRule,omitempty"`
}

type CanvasRect struct {
	CanvasPaint
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

type CanvasLine struct {
	CanvasPaint
	X1 float64 `json:"x1,omitempty"`
	Y1 float64 `json:"y1,omitempty"`
	X2 float64 `json:"x2,omitempty"`
	Y2 float64 `json:"y2,omitempty"`
}

type CanvasArc struct {
	CanvasPaint
	X                float64 `json:"x,omitempty"`
	Y                float64 `json:"y,omitempty"`
	Radius           float64 `json:"radius,omitempty"`
	StartAngle       float64 `json:"startAngle,omitempty"`
	EndAngle         float64 `json:"endAngle,omitempty"`
	CounterClockwise bool    `json:"counterclockwise,omitempty"`
}

type CanvasQuadraticCurve struct {
	CanvasPaint
	X0  float64 `json:"x0,omitempty"`
	Y0  float64 `json:"y0,omitempty"`
	CPX float64 `json:"cpx,omitempty"`
	CPY float64 `json:"cpy,omitempty"`
	X   float64 `json:"x,omitempty"`
	Y   float64 `json:"y,omitempty"`
}

type CanvasBezierCurve struct {
	CanvasPaint
	X0   float64 `json:"x0,omitempty"`
	Y0   float64 `json:"y0,omitempty"`
	CP1X float64 `json:"cp1x,omitempty"`
	CP1Y float64 `json:"cp1y,omitempty"`
	CP2X float64 `json:"cp2x,omitempty"`
	CP2Y float64 `json:"cp2y,omitempty"`
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
}

type CanvasPolyline struct {
	CanvasPaint
	Points []CanvasPoint `json:"points,omitempty"`
}

type CanvasPolygon struct {
	CanvasPaint
	Points []CanvasPoint `json:"points,omitempty"`
}

type CanvasCircle struct {
	CanvasPaint
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Radius float64 `json:"radius,omitempty"`
}

type CanvasEllipse struct {
	CanvasPaint
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	RadiusX  float64 `json:"radiusX,omitempty"`
	RadiusY  float64 `json:"radiusY,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
}

type CanvasPath struct {
	CanvasPaint
	D string `json:"d,omitempty"`
}

type CanvasText struct {
	CanvasPaint
	X        float64  `json:"x,omitempty"`
	Y        float64  `json:"y,omitempty"`
	Text     string   `json:"text,omitempty"`
	MaxWidth *float64 `json:"maxWidth,omitempty"`
}

type CanvasTextBox struct {
	CanvasPaint
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	Width      float64 `json:"width,omitempty"`
	LineHeight float64 `json:"lineHeight,omitempty"`
	Text       string  `json:"text,omitempty"`
}

type CanvasMeasureText struct {
	RequestID string `json:"requestId,omitempty"`
	Text      string `json:"text,omitempty"`
	Font      string `json:"font,omitempty"`
}

type CanvasImage struct {
	CanvasPaint
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

type CanvasImageData struct {
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
	Data   []int   `json:"data,omitempty"`
}

type CanvasSnapshot struct {
	RequestID     string  `json:"requestId,omitempty"`
	MIME          string  `json:"mime,omitempty"`
	Quality       float64 `json:"quality,omitempty"`
	MinIntervalMs int     `json:"minIntervalMs,omitempty"`
}

func (CanvasClear) canvasCommand()          {}
func (CanvasRawCommand) canvasCommand()     {}
func (CanvasSet) canvasCommand()            {}
func (CanvasRegions) canvasCommand()        {}
func (CanvasSave) canvasCommand()           {}
func (CanvasRestore) canvasCommand()        {}
func (CanvasResetTransform) canvasCommand() {}
func (CanvasTranslate) canvasCommand()      {}
func (CanvasScale) canvasCommand()          {}
func (CanvasRotate) canvasCommand()         {}
func (CanvasTransform) canvasCommand()      {}
func (CanvasStyle) canvasCommand()          {}
func (CanvasResetStyle) canvasCommand()     {}
func (CanvasLineDash) canvasCommand()       {}
func (CanvasShadow) canvasCommand()         {}
func (CanvasGradient) canvasCommand()       {}
func (CanvasPattern) canvasCommand()        {}
func (CanvasClip) canvasCommand()           {}
func (CanvasRect) canvasCommand()           {}
func (CanvasLine) canvasCommand()           {}
func (CanvasArc) canvasCommand()            {}
func (CanvasQuadraticCurve) canvasCommand() {}
func (CanvasBezierCurve) canvasCommand()    {}
func (CanvasPolyline) canvasCommand()       {}
func (CanvasPolygon) canvasCommand()        {}
func (CanvasCircle) canvasCommand()         {}
func (CanvasEllipse) canvasCommand()        {}
func (CanvasPath) canvasCommand()           {}
func (CanvasText) canvasCommand()           {}
func (CanvasTextBox) canvasCommand()        {}
func (CanvasMeasureText) canvasCommand()    {}
func (CanvasImage) canvasCommand()          {}
func (CanvasImageData) canvasCommand()      {}
func (CanvasSnapshot) canvasCommand()       {}

func (c CanvasClear) MarshalJSON() ([]byte, error) {
	type payload CanvasClear
	return marshalCanvasCommand(CanvasCommandClear, payload(c), nil)
}

func (c CanvasRawCommand) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(c.Fields)+1)
	for key, value := range c.Fields {
		out[key] = value
	}
	out["type"] = c.Type
	return json.Marshal(out)
}

func (c CanvasSet) MarshalJSON() ([]byte, error) {
	type payload CanvasSet
	return marshalCanvasCommand(CanvasCommandSet, payload(c), nil)
}

func (c CanvasRegions) MarshalJSON() ([]byte, error) {
	type payload CanvasRegions
	return marshalCanvasCommand(CanvasCommandRegions, payload(c), nil)
}

func (c CanvasSave) MarshalJSON() ([]byte, error) {
	type payload CanvasSave
	return marshalCanvasCommand(CanvasCommandSave, payload(c), nil)
}

func (c CanvasRestore) MarshalJSON() ([]byte, error) {
	type payload CanvasRestore
	return marshalCanvasCommand(CanvasCommandRestore, payload(c), nil)
}

func (c CanvasResetTransform) MarshalJSON() ([]byte, error) {
	type payload CanvasResetTransform
	return marshalCanvasCommand(CanvasCommandResetTransform, payload(c), nil)
}

func (c CanvasTranslate) MarshalJSON() ([]byte, error) {
	type payload CanvasTranslate
	return marshalCanvasCommand(CanvasCommandTranslate, payload(c), nil)
}

func (c CanvasScale) MarshalJSON() ([]byte, error) {
	type payload CanvasScale
	return marshalCanvasCommand(CanvasCommandScale, payload(c), nil)
}

func (c CanvasRotate) MarshalJSON() ([]byte, error) {
	type payload CanvasRotate
	return marshalCanvasCommand(CanvasCommandRotate, payload(c), nil)
}

func (c CanvasTransform) MarshalJSON() ([]byte, error) {
	type payload CanvasTransform
	return marshalCanvasCommand(CanvasCommandTransform, payload(c), nil)
}

func (c CanvasStyle) MarshalJSON() ([]byte, error) {
	type payload CanvasStyle
	return marshalCanvasCommand(CanvasCommandStyle, payload(c), &c.CanvasPaint)
}

func (c CanvasResetStyle) MarshalJSON() ([]byte, error) {
	type payload CanvasResetStyle
	return marshalCanvasCommand(CanvasCommandResetStyle, payload(c), nil)
}

func (c CanvasLineDash) MarshalJSON() ([]byte, error) {
	type payload CanvasLineDash
	return marshalCanvasCommand(CanvasCommandLineDash, payload(c), nil)
}

func (c CanvasShadow) MarshalJSON() ([]byte, error) {
	type payload CanvasShadow
	return marshalCanvasCommand(CanvasCommandShadow, payload(c), nil)
}

func (c CanvasGradient) MarshalJSON() ([]byte, error) {
	type payload CanvasGradient
	return marshalCanvasCommand(CanvasCommandGradient, payload(c), nil)
}

func (c CanvasPattern) MarshalJSON() ([]byte, error) {
	type payload CanvasPattern
	return marshalCanvasCommand(CanvasCommandPattern, payload(c), nil)
}

func (c CanvasClip) MarshalJSON() ([]byte, error) {
	type payload CanvasClip
	return marshalCanvasCommand(CanvasCommandClip, payload(c), nil)
}

func (c CanvasRect) MarshalJSON() ([]byte, error) {
	type payload CanvasRect
	return marshalCanvasCommand(CanvasCommandRect, payload(c), &c.CanvasPaint)
}

func (c CanvasLine) MarshalJSON() ([]byte, error) {
	type payload CanvasLine
	return marshalCanvasCommand(CanvasCommandLine, payload(c), &c.CanvasPaint)
}

func (c CanvasArc) MarshalJSON() ([]byte, error) {
	type payload CanvasArc
	return marshalCanvasCommand(CanvasCommandArc, payload(c), &c.CanvasPaint)
}

func (c CanvasQuadraticCurve) MarshalJSON() ([]byte, error) {
	type payload CanvasQuadraticCurve
	return marshalCanvasCommand(CanvasCommandQuadraticCurve, payload(c), &c.CanvasPaint)
}

func (c CanvasBezierCurve) MarshalJSON() ([]byte, error) {
	type payload CanvasBezierCurve
	return marshalCanvasCommand(CanvasCommandBezierCurve, payload(c), &c.CanvasPaint)
}

func (c CanvasPolyline) MarshalJSON() ([]byte, error) {
	type payload CanvasPolyline
	return marshalCanvasCommand(CanvasCommandPolyline, payload(c), &c.CanvasPaint)
}

func (c CanvasPolygon) MarshalJSON() ([]byte, error) {
	type payload CanvasPolygon
	return marshalCanvasCommand(CanvasCommandPolygon, payload(c), &c.CanvasPaint)
}

func (c CanvasCircle) MarshalJSON() ([]byte, error) {
	type payload CanvasCircle
	return marshalCanvasCommand(CanvasCommandCircle, payload(c), &c.CanvasPaint)
}

func (c CanvasEllipse) MarshalJSON() ([]byte, error) {
	type payload CanvasEllipse
	return marshalCanvasCommand(CanvasCommandEllipse, payload(c), &c.CanvasPaint)
}

func (c CanvasPath) MarshalJSON() ([]byte, error) {
	type payload CanvasPath
	return marshalCanvasCommand(CanvasCommandPath, payload(c), &c.CanvasPaint)
}

func (c CanvasText) MarshalJSON() ([]byte, error) {
	type payload CanvasText
	return marshalCanvasCommand(CanvasCommandText, payload(c), &c.CanvasPaint)
}

func (c CanvasTextBox) MarshalJSON() ([]byte, error) {
	type payload CanvasTextBox
	return marshalCanvasCommand(CanvasCommandTextBox, payload(c), &c.CanvasPaint)
}

func (c CanvasMeasureText) MarshalJSON() ([]byte, error) {
	type payload CanvasMeasureText
	return marshalCanvasCommand(CanvasCommandMeasureText, payload(c), nil)
}

func (c CanvasImage) MarshalJSON() ([]byte, error) {
	type payload CanvasImage
	return marshalCanvasCommand(CanvasCommandImage, payload(c), &c.CanvasPaint)
}

func (c CanvasImageData) MarshalJSON() ([]byte, error) {
	type payload CanvasImageData
	return marshalCanvasCommand(CanvasCommandImageData, payload(c), nil)
}

func (c CanvasSnapshot) MarshalJSON() ([]byte, error) {
	type payload CanvasSnapshot
	return marshalCanvasCommand(CanvasCommandSnapshot, payload(c), nil)
}

func marshalCanvasCommand(commandType CanvasCommandType, payload any, paint *CanvasPaint) ([]byte, error) {
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
		paint.applyCanvasPaint(out)
	}
	return json.Marshal(out)
}

func (p CanvasPaint) applyCanvasPaint(out map[string]any) {
	if p.NoFill {
		out["fill"] = false
	}
	if p.NoStroke {
		out["stroke"] = false
	}
}

type CanvasModifierState struct {
	Alt   bool `json:"alt,omitempty"`
	Ctrl  bool `json:"ctrl,omitempty"`
	Meta  bool `json:"meta,omitempty"`
	Shift bool `json:"shift,omitempty"`
}

type CanvasReadyEvent struct {
	Type   CanvasEventType `json:"type"`
	Width  float64         `json:"width"`
	Height float64         `json:"height"`
	DPR    float64         `json:"dpr"`
	Theme  PanelTheme      `json:"theme,omitempty"`
}

func (CanvasReadyEvent) canvasEvent() {}
func (CanvasReadyEvent) EventType() CanvasEventType {
	return CanvasEventReady
}

type CanvasResizeEvent struct {
	Type   CanvasEventType `json:"type"`
	Width  float64         `json:"width"`
	Height float64         `json:"height"`
	DPR    float64         `json:"dpr"`
	Theme  PanelTheme      `json:"theme,omitempty"`
}

func (CanvasResizeEvent) canvasEvent() {}
func (CanvasResizeEvent) EventType() CanvasEventType {
	return CanvasEventResize
}

type CanvasPointerEvent struct {
	Type        CanvasEventType     `json:"type"`
	Event       string              `json:"event"`
	X           float64             `json:"x"`
	Y           float64             `json:"y"`
	Button      int                 `json:"button,omitempty"`
	Buttons     int                 `json:"buttons,omitempty"`
	PointerID   int                 `json:"pointerId,omitempty"`
	PointerType string              `json:"pointerType,omitempty"`
	RegionID    string              `json:"regionId,omitempty"`
	Modifiers   CanvasModifierState `json:"modifiers,omitempty"`
}

func (CanvasPointerEvent) canvasEvent() {}
func (CanvasPointerEvent) EventType() CanvasEventType {
	return CanvasEventPointer
}

type CanvasWheelEvent struct {
	Type      CanvasEventType     `json:"type"`
	X         float64             `json:"x"`
	Y         float64             `json:"y"`
	DeltaX    float64             `json:"deltaX"`
	DeltaY    float64             `json:"deltaY"`
	DeltaMode int                 `json:"deltaMode,omitempty"`
	Modifiers CanvasModifierState `json:"modifiers,omitempty"`
}

func (CanvasWheelEvent) canvasEvent() {}
func (CanvasWheelEvent) EventType() CanvasEventType {
	return CanvasEventWheel
}

type CanvasKeyEvent struct {
	Type      CanvasEventType     `json:"type"`
	Event     string              `json:"event"`
	Key       string              `json:"key"`
	Code      string              `json:"code"`
	Repeat    bool                `json:"repeat,omitempty"`
	Modifiers CanvasModifierState `json:"modifiers,omitempty"`
}

func (CanvasKeyEvent) canvasEvent() {}
func (CanvasKeyEvent) EventType() CanvasEventType {
	return CanvasEventKey
}

type CanvasTextMetricsEvent struct {
	Type                     CanvasEventType `json:"type"`
	RequestID                string          `json:"requestId,omitempty"`
	Text                     string          `json:"text,omitempty"`
	Width                    float64         `json:"width"`
	ActualBoundingBoxLeft    float64         `json:"actualBoundingBoxLeft,omitempty"`
	ActualBoundingBoxRight   float64         `json:"actualBoundingBoxRight,omitempty"`
	ActualBoundingBoxAscent  float64         `json:"actualBoundingBoxAscent,omitempty"`
	ActualBoundingBoxDescent float64         `json:"actualBoundingBoxDescent,omitempty"`
	FontBoundingBoxAscent    float64         `json:"fontBoundingBoxAscent,omitempty"`
	FontBoundingBoxDescent   float64         `json:"fontBoundingBoxDescent,omitempty"`
	EmHeightAscent           float64         `json:"emHeightAscent,omitempty"`
	EmHeightDescent          float64         `json:"emHeightDescent,omitempty"`
	HangingBaseline          float64         `json:"hangingBaseline,omitempty"`
	AlphabeticBaseline       float64         `json:"alphabeticBaseline,omitempty"`
	IdeographicBaseline      float64         `json:"ideographicBaseline,omitempty"`
}

func (CanvasTextMetricsEvent) canvasEvent() {}
func (CanvasTextMetricsEvent) EventType() CanvasEventType {
	return CanvasEventMetrics
}

type CanvasSnapshotEvent struct {
	Type      CanvasEventType `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	MIME      string          `json:"mime,omitempty"`
	DataURL   string          `json:"dataUrl,omitempty"`
	Width     float64         `json:"width,omitempty"`
	Height    float64         `json:"height,omitempty"`
}

func (CanvasSnapshotEvent) canvasEvent() {}
func (CanvasSnapshotEvent) EventType() CanvasEventType {
	return CanvasEventSnapshot
}
