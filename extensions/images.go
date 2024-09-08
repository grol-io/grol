package extensions

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/vector"
	"grol.io/grol/eval"
	"grol.io/grol/object"
)

type GrolImage struct {
	*image.NRGBA
	*vector.Rasterizer
	w, h int
}

type ImageMap map[object.Object]GrolImage

// TODO: make this configurable and use the slice check as well as some sort of LRU.
const MaxImageDimension = 1024 // in pixels.

// HSLToRGB converts HSL values to RGB. h, s and l in [0,1].
func HSLToRGB(h, s, l float64) color.NRGBA {
	var r, g, b float64

	// h = math.Mod(h, 360.) / 360.

	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1. + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+1/3.)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1/3.)
	}

	return color.NRGBA{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
		A: 255,
	}
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1.
	}
	if t > 1 {
		t -= 1.
	}
	if t < 1/6. {
		return p + (q-p)*6*t
	}
	if t < 0.5 {
		return q
	}
	if t < 2/3. {
		return p + (q-p)*(2/3.-t)*6
	}
	return p
}

func hslArrayToRBGAColor(arr []object.Object) (color.NRGBA, *object.Error) {
	rgba := color.NRGBA{}
	if len(arr) != 3 {
		return rgba, object.Errorfp("color array must be [Hue,Saturation,Lightness]")
	}
	var oerr *object.Error
	h, oerr := eval.GetFloatValue(arr[0])
	if oerr != nil {
		return rgba, oerr
	}
	s, oerr := eval.GetFloatValue(arr[1])
	if oerr != nil {
		return rgba, oerr
	}
	l, oerr := eval.GetFloatValue(arr[2])
	if oerr != nil {
		return rgba, oerr
	}
	return HSLToRGB(h, s, l), nil
}

func elem2ColorComponent(o object.Object) (uint8, *object.Error) {
	var i int
	switch o.Type() {
	case object.FLOAT:
		i = int(math.Round(o.(object.Float).Value))
	case object.INTEGER:
		i = int(o.(object.Integer).Value)
	default:
		return 0, object.Errorfp("color component not an integer: %s", o.Inspect())
	}
	if i < 0 || i > 255 {
		return 0, object.Errorfp("color component out of range (should be 0-255): %s", o.Inspect())
	}
	return uint8(i), nil //nolint:gosec // gosec not smart enough to see the range check just above
}

func rgbArrayToRBGAColor(arr []object.Object) (color.NRGBA, *object.Error) {
	rgba := color.NRGBA{}
	if len(arr) < 3 || len(arr) > 4 {
		return rgba, object.Errorfp("color array must be [R,G,B] or [R,G,B,A]")
	}
	var oerr *object.Error
	rgba.R, oerr = elem2ColorComponent(arr[0])
	if oerr != nil {
		return rgba, oerr
	}
	rgba.G, oerr = elem2ColorComponent(arr[1])
	if oerr != nil {
		return rgba, oerr
	}
	rgba.B, oerr = elem2ColorComponent(arr[2])
	if oerr != nil {
		return rgba, oerr
	}
	if len(arr) == 4 {
		rgba.A, oerr = elem2ColorComponent(arr[3])
		if oerr != nil {
			return rgba, oerr
		}
	} else {
		rgba.A = 255
	}
	return rgba, nil
}

func ycbrArrayToRBGAColor(arr []object.Object) (color.NRGBA, *object.Error) {
	rgba := color.NRGBA{}
	ycbcr := color.YCbCr{}
	if len(arr) != 3 {
		return rgba, object.Errorfp("color array must be [Y',Cb,Cr]")
	}
	var oerr *object.Error
	ycbcr.Y, oerr = elem2ColorComponent(arr[0])
	if oerr != nil {
		return rgba, oerr
	}
	ycbcr.Cb, oerr = elem2ColorComponent(arr[1])
	if oerr != nil {
		return rgba, oerr
	}
	ycbcr.Cr, oerr = elem2ColorComponent(arr[2])
	if oerr != nil {
		return rgba, oerr
	}
	rgba.A = 255
	rgba.R, rgba.G, rgba.B = color.YCbCrToRGB(ycbcr.Y, ycbcr.Cb, ycbcr.Cr)
	// return color.YCbCrModel.Convert(ycbcr).(color.NRGBA), nil
	return rgba, nil
}

func createImageFunctions() { //nolint:funlen // this is a group of related functions.
	// All the functions consistently use args[0] as the image name/reference into the ClientData map.
	cdata := make(ImageMap)
	imgFn := object.Extension{
		Name:       "image.new",
		MinArgs:    3,
		MaxArgs:    3,
		Help:       "create a new NRGBA image of the name and size, image starts entirely transparent",
		ArgTypes:   []object.Type{object.STRING, object.INTEGER, object.INTEGER},
		ClientData: cdata,
		Callback: func(cdata any, _ string, args []object.Object) object.Object {
			images := cdata.(ImageMap)
			x := int(args[1].(object.Integer).Value)
			y := int(args[2].(object.Integer).Value)
			if x > MaxImageDimension || y > MaxImageDimension {
				return object.Errorf("image size too large")
			}
			if x < 0 || y < 0 {
				return object.Errorf("image sizes must be positive")
			}
			img := image.NewNRGBA(image.Rect(0, 0, x, y))
			/*
				transparent := color.NRGBA{0, 0, 0, 0}
				draw.Draw(img, img.Bounds(), &image.Uniform{transparent}, image.Point{}, draw.Src)
			*/
			images[args[0]] = GrolImage{img, vector.NewRasterizer(x, y), x, y}
			return args[0]
		},
	}
	MustCreate(imgFn)
	imgFn.Name = "image.set"
	imgFn.Help = "img, x, y, color: set a pixel in the named image, color is an array of 3 or 4 elements 0-255"
	imgFn.MinArgs = 4
	imgFn.MaxArgs = 4
	imgFn.ArgTypes = []object.Type{object.STRING, object.INTEGER, object.INTEGER, object.ARRAY}
	imgFn.Callback = func(cdata any, name string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		x := int(args[1].(object.Integer).Value)
		y := int(args[2].(object.Integer).Value)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image %q not found", args[0].(object.String).Value)
		}
		colorArray := object.Elements(args[3])
		var color color.NRGBA
		var oerr *object.Error
		switch name {
		case "image.set_ycbcr":
			color, oerr = ycbrArrayToRBGAColor(colorArray)
		case "image.set_hsl":
			color, oerr = hslArrayToRBGAColor(colorArray)
		case "image.set":
			color, oerr = rgbArrayToRBGAColor(colorArray)
		default:
			return object.Errorf("unknown image.set function %q", name)
		}
		if oerr != nil {
			return oerr
		}
		img.SetNRGBA(x, y, color)
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image.set_ycbcr"
	imgFn.Help = "img, x, y, color: set a pixel in the named image, color Y'CbCr in an array of 3 elements 0-255"
	MustCreate(imgFn)
	imgFn.Name = "image.set_hsl"
	imgFn.Help = "img, x, y, color: set a pixel in the named image, color in an array [Hue (0-1), Sat (0-1), Light (0-1)]"
	MustCreate(imgFn)
	imgFn.Name = "image.save"
	imgFn.Help = "save the named image grol.png"
	imgFn.MinArgs = 1
	imgFn.MaxArgs = 1
	imgFn.ArgTypes = []object.Type{object.STRING}
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image not found")
		}
		outputFile, err := os.Create("grol.png")
		if err != nil {
			return object.Errorf("error opening image file: %v", err)
		}
		defer outputFile.Close()
		err = png.Encode(outputFile, img.NRGBA)
		if err != nil {
			return object.Errorf("error encoding image: %v", err)
		}
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image.png"
	imgFn.Help = "returns the png data of the named image, suitable for base64"
	imgFn.MinArgs = 1
	imgFn.MaxArgs = 1
	imgFn.ArgTypes = []object.Type{object.STRING}
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image not found")
		}
		buf := bytes.Buffer{}
		err := png.Encode(&buf, img.NRGBA)
		if err != nil {
			return object.Errorf("error encoding image: %v", err)
		}
		return object.String{Value: buf.String()}
	}
	MustCreate(imgFn)
	createVectorImageFunctions(cdata)
}

func createVectorImageFunctions(cdata ImageMap) { //nolint:funlen // this is a group of related functions.
	imgFn := object.Extension{
		Name:       "image.move_to",
		MinArgs:    3,
		MaxArgs:    3,
		Help:       "starts a new path and moves the pen to coords",
		ArgTypes:   []object.Type{object.STRING, object.FLOAT, object.FLOAT},
		ClientData: cdata,
		Callback: func(cdata any, _ string, args []object.Object) object.Object {
			images := cdata.(ImageMap)
			img, ok := images[args[0]]
			if !ok {
				return object.Errorf("image %q not found", args[0].(object.String).Value)
			}
			x := int(args[1].(object.Float).Value)
			y := int(args[2].(object.Float).Value)
			img.Rasterizer.MoveTo(float32(x), float32(y))
			return args[0]
		},
	}
	MustCreate(imgFn)
	imgFn.Name = "image.line_to"
	imgFn.Help = "adds a line segment"
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image %q not found", args[0].(object.String).Value)
		}
		x := int(args[1].(object.Float).Value)
		y := int(args[2].(object.Float).Value)
		img.Rasterizer.LineTo(float32(x), float32(y))
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image.close_path"
	imgFn.Help = "close the current path"
	imgFn.MinArgs = 1
	imgFn.MaxArgs = 1
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image %q not found", args[0].(object.String).Value)
		}
		img.Rasterizer.ClosePath()
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image.draw"
	imgFn.Help = "draw the path in the color is an array of 3 or 4 elements 0-255"
	imgFn.MinArgs = 2
	imgFn.MaxArgs = 2
	imgFn.ArgTypes = []object.Type{object.STRING, object.ARRAY}
	imgFn.Callback = func(cdata any, name string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Errorf("image %q not found", args[0].(object.String).Value)
		}
		colorArray := object.Elements(args[1])
		var color color.NRGBA
		var oerr *object.Error
		switch name {
		case "image.draw_ycbcr":
			color, oerr = ycbrArrayToRBGAColor(colorArray)
		case "image.draw_hsl":
			color, oerr = hslArrayToRBGAColor(colorArray)
		case "image.draw":
			color, oerr = rgbArrayToRBGAColor(colorArray)
		default:
			return object.Errorf("unknown image.draw function %q", name)
		}
		if oerr != nil {
			return oerr
		}
		img.Rasterizer.ClosePath() // just in case
		src := image.NewUniform(color)
		img.Rasterizer.DrawOp = draw.Over
		img.Rasterizer.Draw(img.NRGBA, img.NRGBA.Bounds(), src, image.Point{})
		img.Rasterizer.Reset(img.h, img.w)
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image.draw_ycbcr"
	imgFn.Help = "draw vector path, color Y'CbCr in an array of 3 elements 0-255"
	MustCreate(imgFn)
	imgFn.Name = "image.draw_hsl"
	imgFn.Help = "draw vector path, color in an array [Hue (0-1), Sat (0-1), Light (0-1)]"
	MustCreate(imgFn)
	imgFn.Name = "image.add"
	imgFn.Help = "merges the 2nd image into the first one, additively with white clipping"
	imgFn.ArgTypes = []object.Type{object.STRING, object.STRING}
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img1, ok := images[args[0]]
		if !ok {
			return object.Errorf("image %q not found", args[0].(object.String).Value)
		}
		img2, ok := images[args[1]]
		if !ok {
			return object.Errorf("image %q not found", args[1].(object.String).Value)
		}
		mergeAdd(img1.NRGBA, img2.NRGBA)
		return args[0]
	}
	MustCreate(imgFn)
}

func mergeAdd(img1, img2 *image.NRGBA) {
	//nolint:gosec // gosec not smart enough to see the range check just below.
	for y := range img1.Bounds().Dy() {
		for x := range img1.Bounds().Dx() {
			p1 := img1.NRGBAAt(x, y)
			if p1.R == 0 && p1.G == 0 && p1.B == 0 { // black is no change
				img1.SetNRGBA(x, y, img2.NRGBAAt(x, y))
				continue
			}
			p2 := img2.NRGBAAt(x, y)
			if p2.R == 0 && p2.G == 0 && p2.B == 0 { // black is no change
				continue
			}
			p1.R = uint8(min(255, uint16(p1.R)+uint16(p2.R)))
			p1.G = uint8(min(255, uint16(p1.G)+uint16(p2.G)))
			p1.B = uint8(min(255, uint16(p1.B)+uint16(p2.B)))
			// p1.A = uint8(min(255, uint16(p1.A)+uint16(p2.A))) // summing transparency yield non transparent quickly
			p1.A = max(p1.A, p2.A)
			img1.SetNRGBA(x, y, p1)
		}
	}
}
