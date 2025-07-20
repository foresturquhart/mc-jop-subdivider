# Minecraft Joy of Painting Subdivider

A command-line tool that subdivides large images into optimally-sized tiles for Minecraft's Joy of Painting mod. The tool automatically determines the best canvas sizes for each region and generates both BMP image files and `.paint` files compatible with the mod.

## Features

- **Automatic Canvas Optimization**: Uses the largest possible canvas sizes (32x32, 32x16, 16x32, 16x16) to minimize the number of paintings needed
- **Multiple Format Support**: Accepts BMP, PNG, and JPEG input images
- **Joy of Painting Compatibility**: Generates `.paint` files with proper NBT structure for the Minecraft mod
- **Grid-Based Tiling**: Ensures paintings align perfectly on Minecraft's 16-pixel grid system

## Installation

### Prerequisites

- Go 1.24 or later

### Build from Source

```bash
git clone https://github.com/foresturquhart/mc-jop-subdivider.git
cd mc-jop-subdivider
go build -o mc-jop-subdivider
```

## Usage

### Basic Usage

```bash
./mc-jop-subdivider -input image.png
```

### All Options

```bash
./mc-jop-subdivider \
  -input "path/to/image.png" \
  -author "YourName" \
  -title "My Artwork" \
  -out "output_directory"
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-input` | *required* | Path to input image (BMP, PNG, or JPEG) |
| `-author` | "Unknown" | Author name for .paint files |
| `-title` | "Untitled" | Title for .paint files |
| `-out` | "tiles" | Output directory for generated files |

## Requirements

- **Image Dimensions**: Input images must have dimensions that are multiples of 16 pixels
- **File Formats**: Supports BMP, PNG, and JPEG input formats

## Output

For each tile, the tool generates:

- **`.bmp` file**: The cropped image section
- **`.paint` file**: NBT-encoded data compatible with Joy of Painting mod

### Canvas Sizes

The tool automatically selects the optimal canvas size for each region:

| Canvas Type | Dimensions | Grid Units | Priority |
|-------------|------------|------------|----------|
| Large | 32x32 px | 2x2 units | Highest |
| Wide | 32x16 px | 2x1 units | High |
| Tall | 16x32 px | 1x2 units | Medium |
| Small | 16x16 px | 1x1 units | Lowest |

## Example

```bash
# Convert a 64x48 pixel image
./mc-jop-subdivider -input artwork.png -author "Artist" -title "My Masterpiece"

# Output structure:
tiles/
├── artwork_0_0.bmp    # First tile, row 0
├── artwork_0_0.paint
├── artwork_1_0.bmp    # Second tile, row 0  
├── artwork_1_0.paint
├── artwork_2_0.bmp    # Third tile, row 0
├── artwork_2_0.paint
├── artwork_0_1.bmp    # First tile, row 1
├── artwork_0_1.paint
└── ...
```

## How It Works

1. **Validation**: Checks that image dimensions are multiples of 16
2. **Grid Creation**: Divides the image into 16x16 pixel units
3. **Optimization**: Places the largest possible canvases first to minimize total painting count
4. **Export**: Generates BMP images and NBT-encoded .paint files for each tile

## Dependencies

- [go-mc](https://github.com/Tnze/go-mc) - Minecraft NBT encoding/decoding
- [golang.org/x/image](https://golang.org/x/image) - Extended image format support

## License

See [LICENSE](LICENSE) file for details.
