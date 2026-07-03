package main

import (
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/image/bmp"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

// Embed logos in the binary to retain a single executable.

//go:embed assets/gba_logo_blue.png assets/gba_logo_black.png
var assets embed.FS

// x offset in pixels for the logo
const logoLeft = 58

// BorderFormat is the target console/loader, each of which wants a specific
// BMP bit depth.
type BorderFormat int

const (
	Format8Bpp BorderFormat = iota
	Format15Bpp
	Format24Bpp
)

func readFormatChoice(r io.Reader) (BorderFormat, error) {
	var choice string
	if _, err := fmt.Fscanln(r, &choice); err != nil {
		return 0, err
	}
	switch strings.TrimSpace(choice) {
	case "1":
		return Format8Bpp, nil
	case "2":
		return Format15Bpp, nil
	case "3":
		return Format24Bpp, nil
	default:
		return 0, fmt.Errorf("invalid choice %q (want 1, 2, or 3)", choice)
	}
}

func readLogoChoice(r io.Reader) (image.Image, error) {
	var choice string
	if _, err := fmt.Fscanln(r, &choice); err != nil {
		return nil, err
	}
	var name string
	switch strings.TrimSpace(choice) {
	case "1":
		name = "assets/gba_logo_blue.png"
	case "2":
		name = "assets/gba_logo_black.png"
	case "3":
		return nil, nil // no logo
	default:
		return nil, fmt.Errorf("invalid choice %q (want 1, 2, or 3)", choice)
	}
	f, err := assets.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// applyLogo composites logo onto img flush against the bottom edge, starting
// logoLeft pixels from the left. draw.Over blends the logo's alpha so its
// transparent background lets the border show through.
func applyLogo(img *image.RGBA, logo image.Image) {
	b := img.Bounds()
	lb := logo.Bounds()
	dst := image.Rect(b.Min.X+logoLeft, b.Max.Y-lb.Dy(), b.Min.X+logoLeft+lb.Dx(), b.Max.Y)
	draw.Draw(img, dst, logo, lb.Min, draw.Over)
}

// encodeBorder writes img to w in the BMP bit depth required by format.
func encodeBorder(w io.Writer, img *image.RGBA, format BorderFormat) error {
	switch format {
	case Format15Bpp:
		// x/image/bmp can't emit 15bpp, so use our own RGB555 encoder.
		return encodeBMP15(w, img)
	case Format8Bpp:
		// 8bpp indexed. Plan9 is a generic 256-color palette; swap for a
		// fixed GBARunner3 palette here if you need exact color indices.
		p := image.NewPaletted(img.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(p, p.Bounds(), img, img.Bounds().Min)
		return bmp.Encode(w, p)
	default: // Format24bpp
		// An opaque *image.RGBA makes bmp.Encode emit 24bpp.
		return bmp.Encode(w, img)
	}
}

// encodeBMP15 writes m as a 16-bit BMP using RGB555 bitfields (5 bits each for
// red, green and blue, top bit unused), the format AKMenu/AKAIO expects.
func encodeBMP15(w io.Writer, m image.Image) error {
	b := m.Bounds()
	dx, dy := b.Dx(), b.Dy()

	// Each row is padded to a multiple of 4 bytes.
	rowSize := ((dx*2 + 3) / 4) * 4
	const fileHeaderSize, infoHeaderSize, maskSize = 14, 40, 12
	pixOffset := fileHeaderSize + infoHeaderSize + maskSize
	imageSize := rowSize * dy

	hdr := make([]byte, pixOffset)
	// BITMAPFILEHEADER
	hdr[0], hdr[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(hdr[2:], uint32(pixOffset+imageSize)) // bfSize
	binary.LittleEndian.PutUint32(hdr[10:], uint32(pixOffset))          // bfOffBits
	// BITMAPINFOHEADER
	binary.LittleEndian.PutUint32(hdr[14:], infoHeaderSize)    // biSize
	binary.LittleEndian.PutUint32(hdr[18:], uint32(dx))        // biWidth
	binary.LittleEndian.PutUint32(hdr[22:], uint32(dy))        // biHeight (bottom-up)
	binary.LittleEndian.PutUint16(hdr[26:], 1)                 // biPlanes
	binary.LittleEndian.PutUint16(hdr[28:], 16)                // biBitCount
	binary.LittleEndian.PutUint32(hdr[30:], 3)                 // biCompression = BI_BITFIELDS
	binary.LittleEndian.PutUint32(hdr[34:], uint32(imageSize)) // biSizeImage
	// Color masks for RGB555.
	binary.LittleEndian.PutUint32(hdr[54:], 0x7C00) // red
	binary.LittleEndian.PutUint32(hdr[58:], 0x03E0) // green
	binary.LittleEndian.PutUint32(hdr[62:], 0x001F) // blue
	if _, err := w.Write(hdr); err != nil {
		return err
	}

	// Pixel data is stored bottom-up.
	row := make([]byte, rowSize)
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		off := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := m.At(x, y).RGBA() // each in [0, 0xffff]
			r5 := uint16(r>>11) & 0x1F
			g5 := uint16(g>>11) & 0x1F
			b5 := uint16(bl>>11) & 0x1F
			binary.LittleEndian.PutUint16(row[off:], (r5<<10)|(g5<<5)|b5)
			off += 2
		}
		for ; off < rowSize; off++ {
			row[off] = 0 // padding
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func createImage(input string, output string, format BorderFormat, logo image.Image) error {
	// Open the file for reading. The caller must close it.
	f, fErr := os.Open(input)
	if fErr != nil {
		return fErr
	}
	defer f.Close()

	// Decode the image into a generic image.Image interface.
	var orig image.Image
	var err error
	ext := strings.ToLower(input)
	if strings.HasSuffix(ext, ".bmp") {
		orig, err = bmp.Decode(f)
	} else if strings.HasSuffix(ext, ".png") {
		orig, err = png.Decode(f)
	} else if strings.HasSuffix(ext, ".jpeg") || strings.HasSuffix(ext, ".jpg") {
		orig, err = jpeg.Decode(f)
	} else {
		return errors.New("unsupported file type")
	}
	if err != nil {
		return err
	}

	// Check if image is exactly 256x192
	if orig.Bounds().Dx() != 256 || orig.Bounds().Dy() != 192 {
		return errors.New("image is not exactly 256x192")
	}

	// Allocate a new RGBA canvas matching the original dimensions.
	newImg := image.NewRGBA(orig.Bounds())

	// Copy the original image onto the canvas so we draw on top of it.
	draw.Draw(newImg, newImg.Bounds(), orig, orig.Bounds().Min, draw.Src)

	// GBA display area.
	rect := image.Rect(8, 16, 248, 176)
	c := color.RGBA{255, 0, 0, 255}

	// Copy the solid color into the rectangle using the Src operator.
	draw.Draw(newImg, rect, image.NewUniform(c), image.Point{}, draw.Src)

	// Add GBA logo to the bottom if requested.
	if logo != nil {
		applyLogo(newImg, logo)
	}

	// Create the output file and write the encoded BMP in the chosen format.
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	return encodeBorder(out, newImg, format)
}

func main() {
	fmt.Printf("nds-gba-border-go %s (%s/%s) built %s\n", version, runtime.GOOS, runtime.GOARCH, buildDate)
	if len(os.Args) != 3 {
		log.Fatal("[ERR] Missing arguments\nUsage: ./nds-gba-border-go <input_file> <output_file>")
	}
	input := os.Args[1]
	output := os.Args[2]
	fmt.Println("What do you want to create a border for?\n1) 8bpp (GBARunner3/GBA exploader)\n2) 15bpp (AKMenu/AKMenu-Next)\n3) 24bpp (YSMenu/BootGBA/GBA exploader)")
	format, err := readFormatChoice(os.Stdin)
	if err != nil {
		log.Fatal("[ERR] " + err.Error())
	}
	fmt.Println("Do you want to add the GBA logo to the border?\n1) Yes, blue\n2) Yes, black\n3) No")
	logo, err := readLogoChoice(os.Stdin)
	if err != nil {
		log.Fatal("[ERR] " + err.Error())
	}
	if err := createImage(input, output, format, logo); err != nil {
		log.Fatal("[ERR] " + err.Error())
	}
}
