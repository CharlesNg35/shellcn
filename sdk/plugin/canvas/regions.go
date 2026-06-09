package canvas

type RegionOption func(*Region)

func RectRegion(id string, x, y, width, height float64, opts ...RegionOption) Region {
	region := Region{
		ID: id, Shape: RegionRect,
		X: x, Y: y, Width: width, Height: height,
	}
	applyRegionOptions(&region, opts)
	return region
}

func CircleRegion(id string, x, y, radius float64, opts ...RegionOption) Region {
	region := Region{ID: id, Shape: RegionCircle, X: x, Y: y, Radius: radius}
	applyRegionOptions(&region, opts)
	return region
}

func PolygonRegion(id string, points []Point, opts ...RegionOption) Region {
	region := Region{ID: id, Shape: RegionPolygon, Points: points}
	applyRegionOptions(&region, opts)
	return region
}

func PathRegion(id, d string, opts ...RegionOption) Region {
	region := Region{ID: id, Shape: RegionPath, D: d}
	applyRegionOptions(&region, opts)
	return region
}

func WithCursor(cursor string) RegionOption {
	return func(region *Region) {
		region.Cursor = cursor
	}
}

func WithLabel(label string) RegionOption {
	return func(region *Region) {
		region.Label = label
	}
}

func WithCapturePointer() RegionOption {
	return func(region *Region) {
		region.CapturePointer = true
	}
}

func WithRadius(radius float64) RegionOption {
	return func(region *Region) {
		region.Radius = radius
	}
}

func WithRadii(radii Radii) RegionOption {
	return func(region *Region) {
		region.Radii = &radii
	}
}

func applyRegionOptions(region *Region, opts []RegionOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(region)
		}
	}
}
