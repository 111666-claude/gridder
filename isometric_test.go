package gridder

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	isoTestImageWidth  = 750
	isoTestImageHeight = 400
	isoTestMargin      = 25
	isoTestRows        = 3
	isoTestColumns     = 4
)

func newTestIsometricGrid(t *testing.T, extra ...func(*GridConfig)) *Gridder {
	gridConfig := GridConfig{
		Rows:              isoTestRows,
		Columns:           isoTestColumns,
		MarginWidth:       isoTestMargin,
		LineStrokeWidth:   2,
		BorderStrokeWidth: 4,
		Isometric:         true,
	}
	for _, apply := range extra {
		apply(&gridConfig)
	}

	grid, err := New(
		ImageConfig{Width: isoTestImageWidth, Height: isoTestImageHeight},
		gridConfig,
	)
	assert.Nil(t, err)
	return grid
}

// pixelNear reports whether any pixel in a (2*radius+1) box around
// (x, y) satisfies the given predicate
func pixelNear(img image.Image, x, y, radius int, predicate func(color.Color) bool) bool {
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			if predicate(img.At(x+dx, y+dy)) {
				return true
			}
		}
	}
	return false
}

func isDark(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r < 0x4444 && g < 0x4444 && b < 0x4444
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xcccc && g < 0x3333 && b < 0x3333
}

func TestIsometricConstructionErrors(t *testing.T) {
	_, err := New(ImageConfig{Width: 100, Height: 100}, GridConfig{Rows: 0, Columns: 4, Isometric: true})
	assert.Equal(t, errNoRows, err)

	_, err = New(ImageConfig{Width: 100, Height: 100}, GridConfig{Rows: 3, Columns: 0, Isometric: true})
	assert.Equal(t, errNoColumns, err)

	_, err = New(ImageConfig{Width: 100, Height: 100}, GridConfig{Rows: maxGridDimension + 1, Columns: 4, Isometric: true})
	assert.Equal(t, errGridTooLarge, err)

	_, err = New(ImageConfig{Width: 100, Height: 100}, GridConfig{Rows: 3, Columns: maxGridDimension + 1, Isometric: true})
	assert.Equal(t, errGridTooLarge, err)

	// margin consumes the whole image area
	_, err = New(
		ImageConfig{Width: 10, Height: 10},
		GridConfig{Rows: 3, Columns: 4, MarginWidth: 10, Isometric: true},
	)
	assert.Equal(t, errZeroGridSize, err)

	// the same guard protects the rectangular mode
	_, err = New(
		ImageConfig{Width: 10, Height: 10},
		GridConfig{Rows: 3, Columns: 4, MarginWidth: 10},
	)
	assert.Equal(t, errZeroGridSize, err)
}

func TestIsometricCellDimensions(t *testing.T) {
	grid := newTestIsometricGrid(t)

	// grid area is 700x350 and rows+columns = 7, so each diamond is
	// 200 pixels wide and 100 pixels tall (a non-square 2:1 cell)
	cellWidth, cellHeight := grid.getCellDimensions()
	assert.Equal(t, 200.0, cellWidth)
	assert.Equal(t, 100.0, cellHeight)

	halfWidth, halfHeight := grid.isoHalfSizes()
	assert.Equal(t, 100.0, halfWidth)
	assert.Equal(t, 50.0, halfHeight)
}

func TestIsometricCentersRoundTrip(t *testing.T) {
	grid := newTestIsometricGrid(t)

	for row := 0; row < isoTestRows; row++ {
		for column := 0; column < isoTestColumns; column++ {
			center, err := grid.CellCenter(row, column)
			assert.Nil(t, err)

			hitRow, hitColumn, err := grid.HitTest(center.X, center.Y)
			assert.Nil(t, err)
			assert.Equal(t, row, hitRow)
			assert.Equal(t, column, hitColumn)
		}
	}

	// CellCenter rejects out-of-bounds cells
	_, err := grid.CellCenter(-1, 0)
	assert.Equal(t, errOutOfBounds, err)

	// HitTest rejects image points outside the diamond map
	_, _, err = grid.HitTest(0, 0)
	assert.Equal(t, errOutOfBounds, err)
}

func TestIsometricKnownPoints(t *testing.T) {
	grid := newTestIsometricGrid(t)

	// Hand-computed image coordinates for a 700x350 grid area with a
	// 25px margin, origin (300,50), half-width 100, half-height 50.
	testCases := []struct {
		name   string
		x      float64
		y      float64
		row    int
		column int
	}{
		{name: "center of cell (0,0)", x: 325, y: 75, row: 0, column: 0},
		{name: "center of cell (2,3)", x: 425, y: 325, row: 2, column: 3},
		{name: "center of cell (0,3)", x: 625, y: 225, row: 0, column: 3},
		{name: "center of cell (2,0)", x: 125, y: 175, row: 2, column: 0},
		{name: "north hull tip -> (0,0)", x: 325, y: 25, row: 0, column: 0},
		{name: "east hull tip -> (0,3)", x: 725, y: 225, row: 0, column: 3},
		{name: "south hull tip -> (2,3)", x: 425, y: 375, row: 2, column: 3},
		{name: "west hull tip -> (2,0)", x: 25, y: 175, row: 2, column: 0},
		{name: "shared edge belongs to south cell (1,0)", x: 275, y: 100, row: 1, column: 0},
		{name: "shared edge belongs to east cell (0,1)", x: 375, y: 100, row: 0, column: 1},
		{name: "four-cell corner belongs to (1,1)", x: 325, y: 125, row: 1, column: 1},
	}

	for _, testCase := range testCases {
		row, column, err := grid.HitTest(testCase.x, testCase.y)
		assert.Nil(t, err, testCase.name)
		assert.Equal(t, testCase.row, row, testCase.name)
		assert.Equal(t, testCase.column, column, testCase.name)

		// the same pixel must resolve to the same cell on every query,
		// including shared edges and corners
		for attempt := 0; attempt < 3; attempt++ {
			row2, column2, err2 := grid.HitTest(testCase.x, testCase.y)
			assert.Nil(t, err2, testCase.name)
			assert.Equal(t, row, row2, testCase.name)
			assert.Equal(t, column, column2, testCase.name)
		}
	}

	// points immediately on either side of a shared edge fall in the
	// two cells that border it, so the edge ownership is meaningful
	edgeCases := []struct {
		name   string
		x      float64
		y      float64
		row    int
		column int
	}{
		{name: "just north-west of the shared edge -> (0,0)", x: 274, y: 99, row: 0, column: 0},
		{name: "just south-east of the shared edge -> (1,0)", x: 276, y: 101, row: 1, column: 0},
		{name: "just north-east of the shared edge -> (0,0)", x: 376, y: 99, row: 0, column: 0},
		{name: "just south-west of the shared edge -> (0,1)", x: 374, y: 101, row: 0, column: 1},
	}
	for _, testCase := range edgeCases {
		row, column, err := grid.HitTest(testCase.x, testCase.y)
		assert.Nil(t, err, testCase.name)
		assert.Equal(t, testCase.row, row, testCase.name)
		assert.Equal(t, testCase.column, column, testCase.name)
	}
}

func TestIsometricOutsidePoints(t *testing.T) {
	grid := newTestIsometricGrid(t)
	outsidePoints := []struct {
		name string
		x    float64
		y    float64
	}{
		{name: "negative image coordinates", x: -1000, y: -1000},
		{name: "far below right", x: 100000, y: 100000},
		{name: "inside image but outside the diamond", x: 75, y: 75},
		{name: "just past the east hull tip", x: 740, y: 225},
		{name: "above the north hull tip", x: 325, y: 5},
		{name: "top-left margin corner", x: 0, y: 0},
	}

	for _, point := range outsidePoints {
		_, _, err := grid.HitTest(point.x, point.y)
		assert.Equal(t, errOutOfBounds, err, point.name)
	}

	// non-finite input must fail cleanly instead of panicking
	_, _, err := grid.HitTest(math.NaN(), math.NaN())
	assert.Equal(t, errOutOfBounds, err)
	_, _, err = grid.HitTest(math.Inf(1), math.Inf(-1))
	assert.Equal(t, errOutOfBounds, err)
}

func TestIsometricCustomAndNegativeOrigin(t *testing.T) {
	grid := newTestIsometricGrid(t, func(config *GridConfig) {
		config.OriginX = -50
		config.OriginY = -25
	})

	// cell (0,0) centre is now at local (-50,-25) -> image (-25,0)
	row, column, err := grid.HitTest(-25, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, row)
	assert.Equal(t, 0, column)

	// cell (2,3) centre moved to local (50,225) -> image (75,250)
	row, column, err = grid.HitTest(75, 250)
	assert.Nil(t, err)
	assert.Equal(t, 2, row)
	assert.Equal(t, 3, column)

	_, _, err = grid.HitTest(700, 380)
	assert.Equal(t, errOutOfBounds, err)
}

func TestIsometricEncodePNG(t *testing.T) {
	grid := newTestIsometricGrid(t, func(config *GridConfig) {
		config.BackgroundColor = color.White
		config.LineColor = color.Black
		config.BorderColor = color.Black
	})

	assert.Nil(t, grid.PaintCell(1, 1, color.NRGBA{R: 255, G: 0, B: 0, A: 255}))

	buffer := new(bytes.Buffer)
	assert.Nil(t, grid.EncodePNG(buffer))

	img, _, err := image.Decode(buffer)
	assert.Nil(t, err)

	bounds := img.Bounds()
	assert.Equal(t, isoTestImageWidth, bounds.Dx())
	assert.Equal(t, isoTestImageHeight, bounds.Dy())

	// background is intact in the empty margin corner
	r, g, b, _ := img.At(2, 2).RGBA()
	assert.Equal(t, uint32(0xffff), r)
	assert.Equal(t, uint32(0xffff), g)
	assert.Equal(t, uint32(0xffff), b)

	// border is painted at the north hull tip (325,25)
	assert.True(t, pixelNear(img, 325, 25, 3, isDark))

	// interior grid line is painted at a shared edge midpoint (275,100)
	assert.True(t, pixelNear(img, 275, 100, 3, isDark))

	// painted cell (1,1) is red at its centre: local (300,150) -> image (325,175)
	assert.True(t, pixelNear(img, 325, 175, 2, isRed))
}

func TestIsometricSingleCell(t *testing.T) {
	grid, err := New(
		ImageConfig{Width: 200, Height: 200},
		GridConfig{Rows: 1, Columns: 1, MarginWidth: 10, LineStrokeWidth: 2, BorderStrokeWidth: 4, Isometric: true},
	)
	assert.Nil(t, err)

	// a single diamond fills the 180x180 grid area; its centre is in
	// the middle and all four hull tips resolve to the sole cell
	center, err := grid.CellCenter(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, 100.0, center.X)
	assert.Equal(t, 100.0, center.Y)

	row, column, err := grid.HitTest(center.X, center.Y)
	assert.Nil(t, err)
	assert.Equal(t, 0, row)
	assert.Equal(t, 0, column)

	for _, tip := range [][2]float64{{100, 10}, {190, 100}, {100, 190}, {10, 100}} {
		row, column, err = grid.HitTest(tip[0], tip[1])
		assert.Nil(t, err)
		assert.Equal(t, 0, row)
		assert.Equal(t, 0, column)
	}

	_, _, err = grid.HitTest(15, 15) // inside the image, outside the diamond
	assert.Equal(t, errOutOfBounds, err)
}

func TestIsometricPaintCellBounds(t *testing.T) {
	grid := newTestIsometricGrid(t)

	assert.Equal(t, errOutOfBounds, grid.PaintCell(-1, 0, color.Black))
	assert.Equal(t, errOutOfBounds, grid.PaintCell(0, isoTestColumns, color.Black))
	assert.Nil(t, grid.PaintCell(isoTestRows-1, isoTestColumns-1, color.Black))
}

func TestRectangleModeUnchanged(t *testing.T) {
	grid, err := New(
		ImageConfig{Width: 200, Height: 100},
		GridConfig{Rows: 2, Columns: 2, LineStrokeWidth: 2, BorderStrokeWidth: 4},
	)
	assert.Nil(t, err)

	row, column, err := grid.HitTest(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, 0, row)
	assert.Equal(t, 0, column)

	// (150,50) lies exactly on the shared horizontal and vertical
	// grid lines; int truncation assigns shared lines to the cell
	// below/on the right, which is cell (1,1)
	row, column, err = grid.HitTest(150, 50)
	assert.Nil(t, err)
	assert.Equal(t, 1, row)
	assert.Equal(t, 1, column)

	row, column, err = grid.HitTest(199.9, 99.9)
	assert.Nil(t, err)
	assert.Equal(t, 1, row)
	assert.Equal(t, 1, column)

	_, _, err = grid.HitTest(200, 100)
	assert.Equal(t, errOutOfBounds, err)

	_, _, err = grid.HitTest(-1, 5)
	assert.Equal(t, errOutOfBounds, err)

	center, err := grid.CellCenter(1, 1)
	assert.Nil(t, err)
	assert.Equal(t, 150.0, center.X)
	assert.Equal(t, 75.0, center.Y)

	// the original encode path still works unchanged
	buffer := new(bytes.Buffer)
	assert.Nil(t, grid.EncodePNG(buffer))
}

func TestIsometricLatticeOwnershipExhaustive(t *testing.T) {
	grid := newTestIsometricGrid(t)

	margin := float64(isoTestMargin)

	// Sweep every even-parity lattice vertex in the bounding rectangle.
	// Vertices on or inside the hull diamond resolve to an in-grid cell;
	// vertices outside it must return errOutOfBounds. Repeated queries
	// always return the same answer.
	maxSum := isoTestColumns + isoTestRows - 1
	maxDiff := 2*isoTestRows - 1
	for i := -isoTestRows; i <= isoTestColumns; i++ {
		for j := -1; j <= isoTestRows+isoTestColumns-1; j++ {
			if (i+j)&1 != 0 {
				continue
			}

			sum := i + j
			diff := j - i
			inside := sum >= -1 && sum <= maxSum && diff >= -1 && diff <= maxDiff

			point := grid.latticeToLocal(float64(i), float64(j))
			x := point.X + margin
			y := point.Y + margin

			row, column, err := grid.HitTest(x, y)
			row2, column2, err2 := grid.HitTest(x, y)

			if inside {
				if !assert.Nil(t, err, "inside vertex (%d,%d)", i, j) {
					continue
				}
				assert.True(t, row >= 0 && row < isoTestRows && column >= 0 && column < isoTestColumns)
				assert.Nil(t, err2)
				assert.Equal(t, row, row2)
				assert.Equal(t, column, column2)
			} else {
				assert.Equal(t, errOutOfBounds, err, "outside vertex (%d,%d)", i, j)
				assert.Equal(t, errOutOfBounds, err2)
			}
		}
	}

	// A dense sweep inside the hull must never fail and must return
	// indices covering every cell at least once.
	hit := make(map[[2]int]bool)
	for px := 0.0; px <= 700.0; px += 7 {
		for py := 0.0; py <= 350.0; py += 5 {
			row, column, err := grid.HitTest(px+margin, py+margin)
			if err != nil {
				continue
			}
			hit[[2]int{row, column}] = true

			// the reported centre must map back to the same cell
			center, centerErr := grid.CellCenter(row, column)
			assert.Nil(t, centerErr)
			backRow, backColumn, backErr := grid.HitTest(center.X, center.Y)
			assert.Nil(t, backErr)
			assert.Equal(t, row, backRow)
			assert.Equal(t, column, backColumn)
		}
	}
	assert.Equal(t, isoTestRows*isoTestColumns, len(hit))
}

func TestIsometricDrawHelpersStillWork(t *testing.T) {
	grid := newTestIsometricGrid(t)

	assert.Nil(t, grid.DrawCircle(0, 0, CircleConfig{Radius: 5, Color: color.Black}))
	assert.Nil(t, grid.DrawRectangle(1, 1, RectangleConfig{Width: 10, Height: 10, Color: color.Black}))
	assert.Nil(t, grid.DrawPath(0, 0, 2, 3, PathConfig{Color: color.Black}))
	assert.Equal(t, errOutOfBounds, grid.DrawCircle(isoTestRows, 0))
}
