package main

import (
	"image/color"
	"log"

	"github.com/shomali11/gridder"
)

func main() {
	imageConfig := gridder.ImageConfig{
		Width:  750,
		Height: 400,
		Name:   "example13.png",
	}
	gridConfig := gridder.GridConfig{
		Rows:              3,
		Columns:           4,
		MarginWidth:       25,
		LineStrokeWidth:   2,
		BorderStrokeWidth: 4,
		LineColor:         color.Gray{},
		BorderColor:       color.Black,
		BackgroundColor:   color.NRGBA{R: 220, G: 220, B: 220, A: 255},
		Isometric:         true,
	}

	grid, err := gridder.New(imageConfig, gridConfig)
	if err != nil {
		log.Fatal(err)
	}

	// paint a checkerboard of cells; HitTest lets a level editor map a
	// mouse click back to the row and column under the cursor
	for row := 0; row < gridConfig.Rows; row++ {
		for column := 0; column < gridConfig.Columns; column++ {
			if (row+column)%2 == 0 {
				err = grid.PaintCell(row, column, color.NRGBA{R: 135, G: 211, B: 124, A: 255})
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	// the centre pixel of every cell round-trips through HitTest
	for row := 0; row < gridConfig.Rows; row++ {
		for column := 0; column < gridConfig.Columns; column++ {
			center, err := grid.CellCenter(row, column)
			if err != nil {
				log.Fatal(err)
			}
			hitRow, hitColumn, err := grid.HitTest(center.X, center.Y)
			if err != nil {
				log.Fatal(err)
			}
			if hitRow != row || hitColumn != column {
				log.Fatalf("HitTest mismatch for cell (%d,%d)", row, column)
			}
		}
	}

	if err = grid.SavePNG(); err != nil {
		log.Fatal(err)
	}
}
