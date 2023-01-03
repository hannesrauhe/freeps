package wled

type PixelMatrix [][]string

func MakeDiagonalPixelMatrix(width int, height int, color string, bgcol string) PixelMatrix {
	pm := make([][]string, height)
	for y := 0; y < height; y++ {
		pm[y] = make([]string, width)
		for x := 0; x < width; x++ {
			if x == y {
				pm[y][x] = color
			} else {
				pm[y][x] = bgcol
			}
		}
	}
	return pm
}

func (p PixelMatrix) MoveRight(bgcol string, i int) PixelMatrix {
	if i < 0 {
		return p.MoveLeft(bgcol, i*(-1))
	}

	newpm := make([][]string, len(p))
	for y := 0; y < len(newpm); y++ {
		newpm[y] = make([]string, len(p[0]))
		for x := 0; x < i && x < len(p[0]); x++ {
			newpm[y][x] = bgcol
		}
		for x := 0; x < len(p[0])-i; x++ {
			newpm[y][x+i] = p[y][x]
		}
	}
	return newpm
}

func (p PixelMatrix) MoveLeft(bgcol string, i int) PixelMatrix {
	if i < 0 {
		return p.MoveRight(bgcol, i*(-1))
	}

	newpm := make([][]string, len(p))
	for y := 0; y < len(newpm); y++ {
		newpm[y] = make([]string, len(p[0]))
		for x := 0; x < len(p[0])-i; x++ {
			newpm[y][x] = p[y][x+i]
		}
		for x := len(p[0]) - i; x < len(p[0]); x++ {
			if x < 0 {
				continue
			}
			newpm[y][x] = bgcol
		}
	}
	return newpm
}

func (p PixelMatrix) Shift(i int) PixelMatrix {
	i = i % len(p[0])
	newpm := make([][]string, len(p))
	for y := 0; y < len(newpm); y++ {
		newpm[y] = make([]string, len(p[0]))
		for x := 0; x < len(p[0]); x++ {
			newpm[y][(x+i)%len(p[0])] = p[y][x]
		}
	}
	return newpm
}
