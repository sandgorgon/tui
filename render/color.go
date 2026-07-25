package render

// Color downsampling: truecolor -> 256-color -> 16-color, for
// terminals whose capability level (see term.ColorLevel) doesn't
// support what a cell.Color actually asks for. Distance is plain
// Euclidean RGB distance, not a perceptual metric (CIE76/94/2000) —
// a deliberate simplification; perceptual distance would pick
// marginally better matches at real added complexity for a 16- or
// 256-entry palette, where the difference rarely matters.

// cubeSteps are the per-channel values of xterm's 6x6x6 color cube
// (palette indices 16-231): not a linear 0-255 ramp, this exact
// non-uniform step sequence is what the 256-color palette actually
// uses.
var cubeSteps = [6]int{0, 95, 135, 175, 215, 255}

// ansi16Palette gives the conventional RGB values for the 16 basic
// ANSI colors (xterm's defaults: 0-7 normal, 8-15 bright), used only
// as a reference point for nearest-color downsampling. A real
// terminal's theme may render these differently — there's no way to
// know that from here — but this is the same reference every other
// terminal color-quantization tool uses.
var ansi16Palette = [16][3]uint8{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func colorDistSq(r1, g1, b1, r2, g2, b2 uint8) int {
	dr := int(r1) - int(r2)
	dg := int(g1) - int(g2)
	db := int(b1) - int(b2)
	return dr*dr + dg*dg + db*db
}

func nearestCubeStep(v uint8) int {
	best, bestDist := 0, 1<<30
	for i, s := range cubeSteps {
		d := int(v) - s
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist, best = d, i
		}
	}
	return best
}

// nearestGrayStep finds the closest of the grayscale ramp's 24 steps
// (palette indices 232-255): value = 8 + 10*i for i in [0,24).
func nearestGrayStep(gray int) (idx, val int) {
	best, bestDist := 0, 1<<30
	for i := range 24 {
		v := 8 + 10*i
		d := gray - v
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist, best = d, i
		}
	}
	return best, 8 + 10*best
}

// rgbToIndexed256 maps a truecolor value to the nearest entry in
// xterm's 256-color palette, considering both the 6x6x6 color cube
// (16-231) and the grayscale ramp (232-255) and picking whichever is
// closer.
func rgbToIndexed256(r, g, b uint8) uint8 {
	cr, cg, cb := nearestCubeStep(r), nearestCubeStep(g), nearestCubeStep(b)
	cubeIdx := 16 + 36*cr + 6*cg + cb
	cubeDist := colorDistSq(r, g, b, uint8(cubeSteps[cr]), uint8(cubeSteps[cg]), uint8(cubeSteps[cb]))

	gray := (int(r) + int(g) + int(b)) / 3
	grayIdx, grayVal := nearestGrayStep(gray)
	grayDist := colorDistSq(r, g, b, uint8(grayVal), uint8(grayVal), uint8(grayVal))

	if grayDist < cubeDist {
		return uint8(232 + grayIdx)
	}
	return uint8(cubeIdx)
}

// rgbToIndexed16 maps a truecolor value to the nearest of the 16 basic
// ANSI colors.
func rgbToIndexed16(r, g, b uint8) uint8 {
	best, bestDist := uint8(0), 1<<30
	for i, p := range ansi16Palette {
		d := colorDistSq(r, g, b, p[0], p[1], p[2])
		if d < bestDist {
			bestDist, best = d, uint8(i)
		}
	}
	return best
}

// indexed256ToRGB approximates the RGB value of a 256-color palette
// index, for downsampleIndexedTo16 to work from.
func indexed256ToRGB(n uint8) (r, g, b uint8) {
	switch {
	case n < 16:
		p := ansi16Palette[n]
		return p[0], p[1], p[2]
	case n < 232:
		n -= 16
		return uint8(cubeSteps[n/36]), uint8(cubeSteps[(n/6)%6]), uint8(cubeSteps[n%6])
	default:
		v := uint8(8 + 10*int(n-232))
		return v, v, v
	}
}

// downsampleIndexedTo16 maps a 256-color palette index down to the
// nearest of the 16 basic ANSI colors.
func downsampleIndexedTo16(n uint8) uint8 {
	if n < 16 {
		return n
	}
	r, g, b := indexed256ToRGB(n)
	return rgbToIndexed16(r, g, b)
}
