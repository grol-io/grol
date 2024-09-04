package extensions

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"grol.io/grol/object"
)

type ImageMap map[object.Object]*image.RGBA

// make this configurable and use the slice check as well as some sort of LRU.
const MaxImageSize = 1024

func int2RGBAColor(o object.Object) (uint8, *object.Error) {
	if o.Type() != object.INTEGER {
		return 0, &object.Error{Value: "color component not an integer:" + o.Inspect()}
	}
	i := o.(object.Integer).Value
	if i < 0 || i > 255 {
		return 0, &object.Error{Value: "color component out of range:" + o.Inspect()}
	}
	return uint8(i), nil //nolint:gosec // gosec not smart enough to see the range check just above
}

func arrayToRBGAColor(arr []object.Object) (color.RGBA, *object.Error) {
	rgba := color.RGBA{}
	if len(arr) < 3 || len(arr) > 4 {
		return rgba, &object.Error{Value: "color array must have 3 or 4 elements"}
	}
	var oerr *object.Error
	rgba.R, oerr = int2RGBAColor(arr[0])
	if oerr != nil {
		return rgba, oerr
	}
	rgba.G, oerr = int2RGBAColor(arr[1])
	if oerr != nil {
		return rgba, oerr
	}
	rgba.B, oerr = int2RGBAColor(arr[2])
	if oerr != nil {
		return rgba, oerr
	}
	if len(arr) == 4 {
		rgba.A, oerr = int2RGBAColor(arr[3])
		if oerr != nil {
			return rgba, oerr
		}
	} else {
		rgba.A = 255
	}
	return rgba, nil
}

func createImageFunctions() {
	cdata := make(ImageMap)
	imgFn := object.Extension{
		Name:       "image",
		MinArgs:    3,
		MaxArgs:    3,
		Help:       "create a new image of the name and size",
		ArgTypes:   []object.Type{object.STRING, object.INTEGER, object.INTEGER},
		ClientData: cdata,
		Callback: func(cdata any, _ string, args []object.Object) object.Object {
			images := cdata.(ImageMap)
			x := int(args[1].(object.Integer).Value)
			y := int(args[2].(object.Integer).Value)
			if x > MaxImageSize || y > MaxImageSize {
				return object.Error{Value: "image size too large"}
			}
			if x < 0 || y < 0 {
				return object.Error{Value: "image sizes must be positive"}
			}
			img := image.NewRGBA(image.Rect(0, 0, x, y))
			transparent := color.RGBA{0, 0, 0, 0}
			draw.Draw(img, img.Bounds(), &image.Uniform{transparent}, image.Point{}, draw.Src)
			images[args[0]] = img
			return args[0]
		},
	}
	MustCreate(imgFn)
	imgFn.Name = "image_set"
	imgFn.Help = "set a pixel in the named image, color is an array of 3 or 4 elements 0-255"
	imgFn.MinArgs = 4
	imgFn.MaxArgs = 4
	imgFn.ArgTypes = []object.Type{object.STRING, object.INTEGER, object.INTEGER, object.ARRAY}
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		x := int(args[1].(object.Integer).Value)
		y := int(args[2].(object.Integer).Value)
		img, ok := images[args[0]]
		if !ok {
			return object.Error{Value: "image not found"}
		}
		color, oerr := arrayToRBGAColor(object.Elements(args[3]))
		if oerr != nil {
			return oerr
		}
		img.SetRGBA(x, y, color)
		return args[0]
	}
	MustCreate(imgFn)
	imgFn.Name = "image_save"
	imgFn.Help = "save the named image grol.png"
	imgFn.MinArgs = 1
	imgFn.MaxArgs = 1
	imgFn.ArgTypes = []object.Type{object.STRING}
	imgFn.Callback = func(cdata any, _ string, args []object.Object) object.Object {
		images := cdata.(ImageMap)
		img, ok := images[args[0]]
		if !ok {
			return object.Error{Value: "image not found"}
		}
		outputFile, err := os.Create("grol.png")
		if err != nil {
			return object.Error{Value: "error opening image file: " + err.Error()}
		}
		err = png.Encode(outputFile, img)
		if err != nil {
			return object.Error{Value: "error encoding image: " + err.Error()}
		}
		outputFile.Close()
		return args[0]
	}
	MustCreate(imgFn)
}
