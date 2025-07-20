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
	"strings"
	"time"

	"github.com/Tnze/go-mc/nbt"
	"golang.org/x/image/bmp"
)

// UUID constant for Joy of Painting mod
const paintingUUID = "d1ebe29f-f4e9-4572-83cd-8b2cdbfc2420"

// Config holds CLI configuration and global naming parameters.
type Config struct {
	InputPath string
	Author    string
	Title     string
	OutDir    string
	NameRoot  string
	BaseID    int64
}

// Canvas represents a tile size in pixels and 16px units for placement.
type Canvas struct {
	PxW, PxH       int
	UnitsW, UnitsH int
	CT             byte
}

// Predefined Joy-of-Painting canvas sizes, ordered largest-first.
var canvasTypes = []Canvas{
	{PxW: 32, PxH: 32, UnitsW: 2, UnitsH: 2, CT: 1},
	{PxW: 32, PxH: 16, UnitsW: 2, UnitsH: 1, CT: 2},
	{PxW: 16, PxH: 32, UnitsW: 1, UnitsH: 2, CT: 3},
	{PxW: 16, PxH: 16, UnitsW: 1, UnitsH: 1, CT: 0},
}

// Tile represents a cropped sub-image to export.
type Tile struct {
	Img       image.Image
	CT        byte
	FileBase  string
	TileIndex int
	RowIndex  int
}

// nbtDataStruct defines the structure encoded into .paint files.
type nbtDataStruct struct {
	Generation int32    `nbt:"generation"`
	CT         byte     `nbt:"ct"`
	Pixels     []uint32 `nbt:"pixels"`
	V          int32    `nbt:"v"`
	Author     string   `nbt:"author"`
	Title      string   `nbt:"title"`
	Name       string   `nbt:"name"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// orchestrate flag parsing, planning, and exporting.
func run() error {
	cfg := parseFlags()

	img, err := loadImage(cfg.InputPath)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w%16 != 0 || h%16 != 0 {
		return fmt.Errorf("image dimensions must be multiples of 16: got %dx%d", w, h)
	}

	cols, rows := w/16, h/16
	plan, err := MakeTilePlan(img, rows, cols, cfg.NameRoot)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	var counter int64
	for _, tile := range plan {
		if err := exportTile(cfg, tile, counter); err != nil {
			return fmt.Errorf("exporting tile %q: %w", tile.FileBase, err)
		}
		counter++
		log.Printf("Exported %s (\"%s X %d Y %d\" by %s)", tile.FileBase, cfg.Title, tile.RowIndex, tile.TileIndex, cfg.Author)
	}
	return nil
}

// parseFlags reads CLI arguments and populates Config.
func parseFlags() Config {
	input := flag.String("input", "", "Path to input image (bmp, png, jpeg)")
	author := flag.String("author", "Unknown", "Author name for .paint files")
	title := flag.String("title", "Untitled", "Title for .paint files")
	out := flag.String("out", "tiles", "Output directory for tiles and .paint files")
	flag.Parse()

	if *input == "" {
		log.Fatal("missing input file: use -input <path>")
	}

	base := filepath.Base(*input)
	nameRoot := strings.TrimSuffix(base, filepath.Ext(base))

	return Config{
		InputPath: *input,
		Author:    *author,
		Title:     *title,
		OutDir:    *out,
		NameRoot:  nameRoot,
		BaseID:    time.Now().UnixNano(),
	}
}

// loadImage opens and decodes an image from the disk.
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %q: %w", path, err)
	}
	return img, nil
}

// OccGrid tracks occupied 16×16 cells.
type OccGrid struct {
	rows, cols int
	grid       [][]bool
}

func NewOccGrid(rows, cols int) OccGrid {
	grid := make([][]bool, rows)
	for i := range grid {
		grid[i] = make([]bool, cols)
	}
	return OccGrid{rows: rows, cols: cols, grid: grid}
}

// Empty returns true if the h×w cells from (r,c) are all free.
func (o OccGrid) Empty(r, c, h, w int) bool {
	for y := r; y < r+h; y++ {
		for x := c; x < c+w; x++ {
			if o.grid[y][x] {
				return false
			}
		}
	}
	return true
}

// Mark marks the h×w cells from (r,c) as occupied.
func (o OccGrid) Mark(r, c, h, w int) {
	for y := r; y < r+h; y++ {
		for x := c; x < c+w; x++ {
			o.grid[y][x] = true
		}
	}
}

// MakeTilePlan computes tiling positions for an image.
func MakeTilePlan(img image.Image, rows, cols int, nameRoot string) ([]Tile, error) {
	occ := NewOccGrid(rows, cols)
	var tiles []Tile

	rowIndex := 0
	for r := range rows {
		tileIndex := 0
		hasValidTileInRow := false
		for c := range cols {
			if !occ.Empty(r, c, 1, 1) {
				continue
			}

			// Select largest-fitting canvas
			var sel Canvas
			found := false
			for _, can := range canvasTypes {
				if c+can.UnitsW > cols || r+can.UnitsH > rows {
					continue
				}
				if !occ.Empty(r, c, can.UnitsH, can.UnitsW) {
					continue
				}
				sel = can
				found = true
				break
			}
			if !found {
				return nil, fmt.Errorf("no canvas fits at %d,%d", r, c)
			}

			occ.Mark(r, c, sel.UnitsH, sel.UnitsW)
			x0, y0 := c*16, r*16
			sub := crop(img, x0, y0, sel.PxW, sel.PxH)
			fileBase := fmt.Sprintf("%s_%d_%d", nameRoot, rowIndex, tileIndex)
			tiles = append(tiles, Tile{
				Img:       sub,
				CT:        sel.CT,
				FileBase:  fileBase,
				TileIndex: tileIndex,
				RowIndex:  rowIndex,
			})
			hasValidTileInRow = true
			tileIndex++
		}
		if hasValidTileInRow {
			rowIndex++
		}
	}

	return tiles, nil
}

// exportTile writes BMP and .paint files for a Tile.
func exportTile(cfg Config, tile Tile, counter int64) error {
	// BMP
	bmpPath := filepath.Join(cfg.OutDir, tile.FileBase+".bmp")
	if err := writeBMP(bmpPath, tile.Img); err != nil {
		return err
	}

	// Build pixel data
	h := tile.Img.Bounds().Dy()
	w := tile.Img.Bounds().Dx()
	pixels := make([]uint32, w*h)
	alpha := uint32(0xFF) << 24
	idx := 0
	for y := range h {
		for x := range w {
			r8, g8, b8, _ := tile.Img.At(x, y).RGBA()
			pixels[idx] = alpha |
				uint32(uint8(r8>>8))<<16 |
				uint32(uint8(g8>>8))<<8 |
				uint32(uint8(b8>>8))
			idx++
		}
	}

	// NBT
	nbtData := nbtDataStruct{
		Generation: 1,
		CT:         tile.CT,
		Pixels:     pixels,
		V:          2,
		Author:     cfg.Author,
		Title:      cfg.Title,
		Name:       fmt.Sprintf("%s_%d", paintingUUID, cfg.BaseID+counter),
	}
	paintPath := filepath.Join(cfg.OutDir, tile.FileBase+".paint")
	if err := writePaint(paintPath, nbtData); err != nil {
		return err
	}

	return nil
}

// writeBMP encodes and writes an image as BMP.
func writeBMP(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %q: %w", path, err)
	}
	defer f.Close()
	if err := bmp.Encode(f, img); err != nil {
		return fmt.Errorf("encoding bmp: %w", err)
	}
	return nil
}

// writePaint encodes and writes NBT data wrapped in gzip.
func writePaint(path string, data nbtDataStruct) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %q: %w", path, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	if err := nbt.NewEncoder(gw).Encode(data, ""); err != nil {
		return fmt.Errorf("encoding nbt: %w", err)
	}
	return nil
}

// crop returns an RGBA sub-image of given dimensions.
func crop(img image.Image, x, y, w, h int) image.Image {
	r := image.Rect(0, 0, w, h)
	sub := image.NewRGBA(r)
	dx := image.Rect(x, y, x+w, y+h)
	draw.Draw(sub, r, img, dx.Min, draw.Src)
	return sub
}
