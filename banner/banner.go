package banner

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed banner.txt
var banner string

func ShowTheBanner() {
	lines := strings.Split(banner, "\n")
	printGradientBanner(lines)
}

func printGradientBanner(lines []string) {
	total := 0
	for _, line := range lines {
		total += len(line)
	}

	index := 0
	for _, line := range lines {
		for _, char := range line {
			r, g := gradientColor(index, total)
			fmt.Printf("\033[38;2;%d;%d;0m%c", r, g, char)
			index++
		}
		fmt.Println()
	}
	fmt.Print("\033[0m")
}

func gradientColor(position, max int) (int, int) {
	if max == 0 {
		return 255, 0
	}

	r := 255
	g := int(float64(position) / float64(max) * 230)
	if g > 230 {
		g = 230
	}
	return r, g
}
