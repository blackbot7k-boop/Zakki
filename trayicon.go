package main

// 托盘图标：程序化生成一张 32×32 的 PNG，7 个彩色圆点排成蜂窝，
// 与应用里的七个点呼应。不引入任何图像资源文件。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

func trayPNG() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// 蜂窝排布：上 2、中 3、下 2。
	type dot struct{ x, y float64 }
	positions := []dot{
		{10, 8}, {22, 8},
		{6, 16}, {16, 16}, {26, 16},
		{10, 24}, {22, 24},
	}
	const radius = 4.6

	fill := func(x, y int, c color.RGBA) {
		i := img.PixOffset(x, y)
		img.Pix[i] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			for i, p := range positions {
				dx, dy := float64(x)-p.x, float64(y)-p.y
				d := math.Sqrt(dx*dx + dy*dy)
				if d <= radius {
					uc := dotColor(i)
					fill(x, y, color.RGBA{R: uc.R, G: uc.G, B: uc.B, A: uc.A})
					break
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
