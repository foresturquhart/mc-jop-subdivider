package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Tnze/go-mc/nbt"
	"github.com/google/uuid"
	"golang.org/x/image/bmp"
)

// Allowed tile sizes (pixels) and their corresponding Joy of Painting "ct" values
// Ordered largest-first to maximize use of big canvases
var canvasTypes = []struct {
	w, h int
	ct   byte
}{
	{32, 32, 1}, // LARGE (area=1024)
	{32, 16, 2}, // LONG  (area=512)
	{16, 32, 3}, // TALL  (area=512)
	{16, 16, 0}, // SMALL (area=256)
}

func main() {
	// CLI flags
	inputPath := flag.String("input", "", "Path to input image (bmp, png, jpeg)")
	author := flag.String("author", "Unknown", "Author name for .paint files")
	title := flag.String("title", "Untitled", "Title for .paint files")
	outDir := flag.String("out", "tiles", "Output directory for tiles and .paint files")
	flag.Parse()

	if *inputPath == "" {
		log.Fatal("Missing input file: use -input <path>")
	}

	// Open and decode source image
	f, err := os.Open(*inputPath)
	if err != nil {
		log.Fatalf("Failed to open input image: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		log.Fatalf("Failed to decode image: %v", err)
	}

	// Bounds and divisibility check
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w%16 != 0 || h%16 != 0 {
		log.Fatalf("Image dimensions must be multiples of 16: got %dx%d", w, h)
	}

	cols, rows := w/16, h/16

	// occupancy grid
	occ := make([][]bool, rows)
	for i := range occ {
		occ[i] = make([]bool, cols)
	}

	// Prepare output directory
	err = os.MkdirAll(*outDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create output dir: %v", err)
	}

	// Prepare base UUID and ID counter
	baseUUID := uuid.MustParse("d1ebe29f-f4e9-4572-83cd-8b2cdbfc2420").String()
	baseID := time.Now().Unix()
	counter := int64(0)

	// Iterate over grid, greedy largest-first packing
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if occ[r][c] {
				continue
			}

			// Select the largest fitting canvas
			var sel struct {
				wUnits, hUnits, wPx, hPx int
				ct                       byte
			}
			found := false
			for _, can := range canvasTypes {
				wUnits := can.w / 16
				hUnits := can.h / 16
				// Check bounds
				if c+wUnits > cols || r+hUnits > rows {
					continue
				}
				// Check occupancy
				hit := false
				for y := r; y < r+hUnits && !hit; y++ {
					for x := c; x < c+wUnits; x++ {
						if occ[y][x] {
							hit = true
							break
						}
					}
				}
				if hit {
					continue
				}
				// Choose this canvas
				sel = struct {
					wUnits, hUnits, wPx, hPx int
					ct                       byte
				}{
					wUnits, hUnits, can.w, can.h, can.ct,
				}
				found = true
				break
			}
			if !found {
				log.Fatalf("No supported Joy-of-Painting canvas fits at %d,%d", r, c)
			}

			// Mark occupied cells
			for y := r; y < r+sel.hUnits; y++ {
				for x := c; x < c+sel.wUnits; x++ {
					occ[y][x] = true
				}
			}

			// Crop and export subimage
			x0, y0 := c*16, r*16
			sub := crop(img, x0, y0, sel.wPx, sel.hPx)

			base := filepath.Base(*inputPath)
			nameBase := fmt.Sprintf("%s_%d_%d", trimExt(base), c, r)

			// Write BMP
			bmpFile, _ := os.Create(filepath.Join(*outDir, nameBase+".bmp"))
			bmp.Encode(bmpFile, sub)
			bmpFile.Close()

			// Build a pixel array for .paint
			pixels := make([]int32, sel.wPx*sel.hPx)
			for yy := 0; yy < sel.hPx; yy++ {
				for xx := 0; xx < sel.wPx; xx++ {
					r, g, b, _ := sub.At(xx, yy).RGBA()
					r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
					pixels[yy*sel.wPx+xx] = int32(0xFF<<24 | int(r8)<<16 | int(g8)<<8 | int(b8))
				}
			}

			// Assemble NBT data
			nbtData := map[string]interface{}{
				"generation": int32(1),
				"ct":         sel.ct,
				"pixels":     pixels,
				"v":          int32(2),
				"author":     *author,
				"title":      *title,
				"name":       fmt.Sprintf("%s_%d", baseUUID, baseID+counter),
			}
			counter++

			// Write .paint (NBT gzip)
			pf, _ := os.Create(filepath.Join(*outDir, nameBase+".paint"))
			gw := gzip.NewWriter(pf)
			nbt.NewEncoder(gw).Encode(nbtData, "")
			gw.Close()
			pf.Close()

			log.Printf("Exported %s (bmp + paint)", nameBase)
		}
	}
}

// trimExt removes the file extension
func trimExt(fname string) string {
	ext := filepath.Ext(fname)
	return fname[:len(fname)-len(ext)]
}

// crop returns an RGBA subimage of given dimensions
func crop(img image.Image, x, y, w, h int) image.Image {
	r := image.Rect(0, 0, w, h)
	sub := image.NewRGBA(r)
	dx := image.Rect(x, y, x+w, y+h)
	draw.Draw(sub, r, img, dx.Min, draw.Src)
	return sub
}
