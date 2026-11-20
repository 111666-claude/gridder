package gridder

import (
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

// Isometric geometry
//
// An isometric grid is made of diamond-shaped tiles. A tile's lattice
// position is described by integer coordinates (i, j) in a skewed
// coordinate system. The mapping from lattice coordinates to grid-local
// pixels (the image area inside the margin) is:
//
//	x = OriginX + i * halfWidth
//	y = OriginY + j * halfHeight
//
// Cell (row, column) occupies lattice centre (column-row, column+row),
// so neighbouring tiles share full edges and tile the plane without
// gaps or overlaps:
//
//	north: (column-row, column+row-1)
//	east : (column-row+1, column+row)
//	south: (column-row, column+row+1)
//	west : (column-row-1, column+row)
//
// The origin (OriginX, OriginY) is the centre of cell (0, 0). When the
// configured origin is (0, 0) the diamond map is centred automatically,
// fitting its bounding diamond exactly inside the grid area. Negative
// origins are supported and simply shift the map up and left.

// isoHalfSizes returns the horizontal and vertical semi-axes of a
// diamond cell. With R rows and C columns the map's bounding diamond
// spans (R+C) half-widths horizontally and (R+C) half-heights vertically,
// so non-square cells follow naturally from non-square images.
func (g *Gridder) isoHalfSizes() (float64, float64) {
	gridWidth, gridHeight := g.getGridDimensions()
	denominator := float64(g.gridConfig.GetRows() + g.gridConfig.GetColumns())
	return gridWidth / denominator, gridHeight / denominator
}

// getIsometricCellDimensions returns the full diamond width and height
func (g *Gridder) getIsometricCellDimensions() (float64, float64) {
	halfWidth, halfHeight := g.isoHalfSizes()
	return 2 * halfWidth, 2 * halfHeight
}

// getIsometricOrigin returns the grid-local pixel position of the
// centre of cell (0, 0), applying the automatic centring default
func (g *Gridder) getIsometricOrigin() (float64, float64) {
	originX := g.gridConfig.GetOriginX()
	originY := g.gridConfig.GetOriginY()
	if originX != 0 || originY != 0 {
		return originX, originY
	}

	halfWidth, halfHeight := g.isoHalfSizes()
	rows := float64(g.gridConfig.GetRows())

	// Shift so that the west hull vertex sits on x = 0 and the north
	// hull vertex sits on y = 0, centring the map in the grid area.
	return rows * halfWidth, halfHeight
}

// latticeToLocal converts a lattice point to grid-local pixel coordinates
func (g *Gridder) latticeToLocal(i, j float64) gg.Point {
	originX, originY := g.getIsometricOrigin()
	halfWidth, halfHeight := g.isoHalfSizes()
	return gg.Point{X: originX + i*halfWidth, Y: originY + j*halfHeight}
}

// getIsometricCellCenter returns the grid-local centre of a cell
func (g *Gridder) getIsometricCellCenter(row, column int) *gg.Point {
	i := float64(column - row)
	j := float64(column + row)
	point := g.latticeToLocal(i, j)
	return &point
}

// isoCellVertices returns the north, east, south and west vertices of a cell
func (g *Gridder) isoCellVertices(row, column int) [4]gg.Point {
	i := float64(column - row)
	j := float64(column + row)
	return [4]gg.Point{
		g.latticeToLocal(i, j-1), // north
		g.latticeToLocal(i+1, j), // east
		g.latticeToLocal(i, j+1), // south
		g.latticeToLocal(i-1, j), // west
	}
}

// isoHullVertices returns the north, east, south and west vertices of
// the outer boundary of the whole diamond map
func (g *Gridder) isoHullVertices() [4]gg.Point {
	rows := float64(g.gridConfig.GetRows())
	columns := float64(g.gridConfig.GetColumns())
	return [4]gg.Point{
		g.latticeToLocal(0, -1),                        // north
		g.latticeToLocal(columns, columns-1),           // east
		g.latticeToLocal(columns-rows, rows+columns-1), // south
		g.latticeToLocal(-rows, rows-1),                // west
	}
}

// paintIsometricGrid draws the interior diamond edges. Each edge family
// is collinear on screen, so the whole grid needs only (rows-1)+(columns-1)
// straight segments regardless of the number of cells.
func (g *Gridder) paintIsometricGrid() {
	rows := g.gridConfig.GetRows()
	columns := g.gridConfig.GetColumns()

	g.ctx.Push()
	g.ctx.NewSubPath()

	// Row boundaries lie on lattice lines j - i = 2r-1, r = 1..rows-1.
	// They run from the north-west hull edge (i+j = -1) to the
	// south-east hull edge (i+j = 2*columns-1).
	for r := 1; r < rows; r++ {
		fr := float64(r)
		fc := float64(columns)
		start := g.latticeToLocal(-fr, fr-1)
		end := g.latticeToLocal(fc-fr, fc+fr-1)
		g.ctx.MoveTo(start.X, start.Y)
		g.ctx.LineTo(end.X, end.Y)
	}

	// Column boundaries lie on lattice lines i + j = 2c-1, c = 1..columns-1.
	// They run from the north-east hull edge (j-i = -1) to the
	// south-west hull edge (j-i = 2*rows-1).
	for c := 1; c < columns; c++ {
		fc := float64(c)
		fr := float64(rows)
		start := g.latticeToLocal(fc, fc-1)
		end := g.latticeToLocal(fc-fr, fc+fr-1)
		g.ctx.MoveTo(start.X, start.Y)
		g.ctx.LineTo(end.X, end.Y)
	}

	if dashes := g.gridConfig.GetLineDashes(); dashes > 0 {
		g.ctx.SetDash(dashes)
	} else {
		g.ctx.SetDash()
	}
	g.ctx.SetColor(g.gridConfig.GetLineColor())
	g.ctx.SetLineWidth(g.gridConfig.GetLineStrokeWidth())
	g.ctx.Stroke()
	g.ctx.Pop()
}

// paintIsometricBorder traces the outer diamond hull using the border
// stroke, dash and colour configuration
func (g *Gridder) paintIsometricBorder() {
	hull := g.isoHullVertices()

	g.ctx.Push()
	g.ctx.NewSubPath()
	g.ctx.MoveTo(hull[0].X, hull[0].Y)
	for _, vertex := range hull[1:] {
		g.ctx.LineTo(vertex.X, vertex.Y)
	}
	g.ctx.ClosePath()

	if dashes := g.gridConfig.GetBorderDashes(); dashes > 0 {
		g.ctx.SetDash(dashes)
	} else {
		g.ctx.SetDash()
	}
	g.ctx.SetLineWidth(g.gridConfig.GetBorderStrokeWidth())
	g.ctx.SetColor(g.gridConfig.GetBorderColor())
	g.ctx.Stroke()
	g.ctx.Pop()
}

// paintIsometricCell fills a diamond cell. The diamond is scaled
// uniformly towards its centre by half the line stroke width (measured
// against the smaller semi-axis) so the fill stays inside the grid
// lines even for non-square cells
func (g *Gridder) paintIsometricCell(row, column int, fillColor color.Color) error {
	vertices := g.isoCellVertices(row, column)
	halfWidth, halfHeight := g.isoHalfSizes()

	center := g.getIsometricCellCenter(row, column)
	inset := g.gridConfig.GetLineStrokeWidth() / 2
	scale := 1.0 - inset/math.Min(halfWidth, halfHeight)
	if scale < 0 {
		scale = 0
	}

	g.ctx.Push()
	g.ctx.NewSubPath()
	for index, vertex := range vertices {
		point := gg.Point{
			X: center.X + (vertex.X-center.X)*scale,
			Y: center.Y + (vertex.Y-center.Y)*scale,
		}
		if index == 0 {
			g.ctx.MoveTo(point.X, point.Y)
		} else {
			g.ctx.LineTo(point.X, point.Y)
		}
	}
	g.ctx.ClosePath()
	g.ctx.SetColor(fillColor)
	g.ctx.Fill()
	g.ctx.Pop()
	return nil
}

// hitTestRectangle maps an image pixel to a rectangular-grid cell
func (g *Gridder) hitTestRectangle(x, y float64) (int, int, error) {
	margin := float64(g.gridConfig.GetMarginWidth())
	cellWidth, cellHeight := g.getCellDimensions()
	gridWidth, gridHeight := g.getGridDimensions()

	localX := x - margin
	localY := y - margin
	if localX < 0 || localY < 0 || localX >= gridWidth || localY >= gridHeight {
		return 0, 0, errOutOfBounds
	}

	row := int(localY / cellHeight)
	column := int(localX / cellWidth)
	if err := g.verifyInBounds(row, column); err != nil {
		return 0, 0, err
	}
	return row, column, nil
}

// hitTestIsometric maps an image pixel to an isometric-grid cell.
//
// The inverse lattice transform gives continuous (i, j) coordinates.
// Cell (r, c) is the diamond 2r-1 <= j-i <= 2r+1 and
// 2c-1 <= i+j <= 2c+1, so the indices come from snapFloor of the
// transformed coordinates. snapFloor treats values sitting on a shared
// edge or corner (within a relative floating-point tolerance) as the
// exact integer lattice value, which gives every border point one
// stable owner: edges belong to the cell on their south/east side and
// corners to the cell whose north corner they are. Indices are then
// clamped to the grid range and the diamond inequality re-checked, so
// hull tips hit their touching cell while every point outside the map
// returns errOutOfBounds.
func (g *Gridder) hitTestIsometric(x, y float64) (int, int, error) {
	margin := float64(g.gridConfig.GetMarginWidth())
	originX, originY := g.getIsometricOrigin()
	halfWidth, halfHeight := g.isoHalfSizes()

	i := (x - margin - originX) / halfWidth
	j := (y - margin - originY) / halfHeight
	if math.IsNaN(i) || math.IsNaN(j) || math.IsInf(i, 0) || math.IsInf(j, 0) {
		return 0, 0, errOutOfBounds
	}

	a := j - i
	b := i + j
	epsilon := 1e-9 * math.Max(1.0, math.Max(math.Abs(a), math.Abs(b)))

	rows := g.gridConfig.GetRows()
	columns := g.gridConfig.GetColumns()

	row := snapFloor((a+1)/2, epsilon)
	column := snapFloor((b+1)/2, epsilon)

	// Clamp so the four sharp corners of the outer hull, whose exact
	// lattice coordinates floor to an out-of-range index, still resolve
	// to the single in-grid cell they touch. The diamond inequality
	// below rejects every other point that only landed here by clamping.
	if row < 0 {
		row = 0
	} else if row >= rows {
		row = rows - 1
	}
	if column < 0 {
		column = 0
	} else if column >= columns {
		column = columns - 1
	}

	fr := float64(row)
	fc := float64(column)
	if a < 2*fr-1-epsilon || a > 2*fr+1+epsilon ||
		b < 2*fc-1-epsilon || b > 2*fc+1+epsilon {
		return 0, 0, errOutOfBounds
	}
	return row, column, nil
}

// snapFloor is math.Floor with a tolerance so that values that should be
// exact integers but land slightly below them due to floating-point
// arithmetic still round to that integer
func snapFloor(value, epsilon float64) int {
	return int(math.Floor(value + epsilon))
}
