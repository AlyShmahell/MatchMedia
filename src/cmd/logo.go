package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

func decodeLogo() (image.Image, error) {
	return png.Decode(bytes.NewReader(logoPNG))
}

func renderLogo(img image.Image) string {
	if img == nil {
		return ""
	}
	b := img.Bounds()
	var sb strings.Builder
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x++ {
			top := rgbaAt(img, x, y)
			bot := color.RGBA{}
			if y+1 < b.Max.Y {
				bot = rgbaAt(img, x, y+1)
			}
			sb.WriteString(halfBlock(top, bot))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func rgbaAt(img image.Image, x, y int) color.RGBA {
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func halfBlock(top, bot color.RGBA) string {
	topOn := top.A > 0
	botOn := bot.A > 0
	if !topOn && !botOn {
		return " "
	}
	var s strings.Builder
	s.WriteString("\x1b[")
	if topOn {
		fmt.Fprintf(&s, "38;2;%d;%d;%d", top.R, top.G, top.B)
	}
	if botOn {
		if topOn {
			s.WriteByte(';')
		}
		fmt.Fprintf(&s, "48;2;%d;%d;%d", bot.R, bot.G, bot.B)
	}
	s.WriteString("m▀\x1b[0m")
	return s.String()
}
