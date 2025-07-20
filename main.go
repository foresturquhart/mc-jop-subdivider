package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"

	"golang.org/x/image/bmp"
)

// tileSize defines an allowed tile in unit and pixel dimensions
type tileSize struct {
	wUnits, hUnits int // in 16px units
	wPx, hPx       int // in pixels
}

var tileSizes = []tileSize{
	// Sorted by area (units) descending -> bigger first
	{4, 4, 64, 64}, // 64x64
	{3, 4, 48, 64}, // 48x64
	{4, 3, 64, 48}, // 64x48
	{3, 3, 48, 48}, // 48x48
	{4, 2, 64, 32}, // 64x32
	{2, 2, 32, 32}, // 32x32
	{2, 1, 32, 16}, // 32x16
	{1, 2, 16, 32}, // 16x32
	{1, 1, 16, 16}, // 16x16
}

func main() {
	// Parse flags
	inputPath := flag.String("input", "", "Path to input image (bmp, png, jpeg)")
	outDir := flag.String("out", "tiles", "Output directory for tiles")
	flag.Parse()

	if *inputPath == "" {
		log.Fatal("Missing input file: use -input <path>")
	}

	// Open input file
	f, err := os.Open(*inputPath)
	if err != nil {
		log.Fatalf("Failed to open input image: %v", err)
	}
	defer f.Close()

	// Decode image
	img, format, err := image.Decode(f)
	if err != nil {
		log.Fatalf("Failed to decode image: %v", err)
	}
	log.Printf("Decoded image format: %s", format)

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > 512 || height > 512 {
		log.Fatalf("Image dimensions exceed 512x512: got %dx%d", width, height)
	}
	// Check divisibility by 16
	if width%16 != 0 || height%16 != 0 {
		log.Fatalf("Image dimensions must be multiples of 16: got %dx%d", width, height)
	}

	cols := width / 16
	rows := height / 16

	// occupancy grid: false = free, true = filled
	occ := make([][]bool, rows)
	for i := range occ {
		occ[i] = make([]bool, cols)
	}

	// Create output directory if not exists
	err = os.MkdirAll(*outDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Iterate over grid
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if occ[r][c] {
				continue // already filled
			}
			// Find largest tile that fits
			var chosen tileSize
			found := false
			for _, t := range tileSizes {
				// Check bounds
				if c+t.wUnits > cols || r+t.hUnits > rows {
					continue
				}
				// Check occupancy
				hit := false
				for y := r; y < r+t.hUnits && !hit; y++ {
					for x := c; x < c+t.wUnits; x++ {
						if occ[y][x] {
							hit = true
							break
						}
					}
				}
				if hit {
					continue
				}
				// This tile fits
				chosen = t
				found = true
				break
			}
			if !found {
				log.Fatalf("No tile fits at grid cell %d,%d", r, c)
			}
			// Mark occupancy
			for y := r; y < r+chosen.hUnits; y++ {
				for x := c; x < c+chosen.wUnits; x++ {
					occ[y][x] = true
				}
			}
			// Crop subimage
			x0 := c * 16
			y0 := r * 16
			sub := crop(img, x0, y0, chosen.wPx, chosen.hPx)

			// Save as BMP
			outPath := fmt.Sprintf("%s/tile_r%dc%d_%dx%d.bmp", *outDir, r, c, chosen.wPx, chosen.hPx)
			outF, err := os.Create(outPath)
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}
			if err := bmp.Encode(outF, sub); err != nil {
				outF.Close()
				log.Fatalf("Failed to encode BMP: %v", err)
			}
			outF.Close()
			log.Printf("Wrote tile: %s", outPath)
		}
	}
}

// crop returns an RGBA subimage of given dimensions
func crop(img image.Image, x, y, w, h int) image.Image {
	r := image.Rect(0, 0, w, h)
	sub := image.NewRGBA(r)
	dx := image.Rect(x, y, x+w, y+h)
	draw.Draw(sub, r, img, dx.Min, draw.Src)
	return sub
}
