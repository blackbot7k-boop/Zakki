package app

// 菜单栏图标：透明底白色单圆环。
// 36px 画布由 macOS 缩到 18pt，留 1pt 透明边距；超采样避免圆边锯齿。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

func trayPNG() []byte {
	const size = 36
	const samples = 4
	const outer = float64(size)/2 - 2
	const inner = outer * (1 - 2*2.5/18)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			covered := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					dx := float64(x) + (float64(sx)+0.5)/samples - float64(size)/2
					dy := float64(y) + (float64(sy)+0.5)/samples - float64(size)/2
					d2 := dx*dx + dy*dy
					if d2 >= inner*inner && d2 <= outer*outer {
						covered++
					}
				}
			}
			if covered > 0 {
				alpha := uint8((covered*255 + samples*samples/2) / (samples * samples))
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
