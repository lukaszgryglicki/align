package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func optimizeAlignment(img1, img2 image.Image, fromX, fromY, rangeX, rangeY, sizeX, sizeY, thrN int) (int, int, float64) {
	if thrN%3 == 0 {
		thrN = (thrN / 3) + 1
	} else {
		thrN = thrN / 3
	}
	ch := make(chan struct{})
	nThreads := 0
	startX := fromX - sizeX
	endX := fromX + sizeX
	startY := fromY - sizeY
	endY := fromY + sizeY
	mtx := &sync.Mutex{}
	minDist, minOffX, minOffY := float64(1e10), 0, 0
	for oXi := -rangeX; oXi <= rangeX; oXi++ {
		go func(ch chan struct{}, oX int) {
			for oY := -rangeY; oY <= rangeY; oY++ {
				// calc distance for offset (oX, oY)
				dist := 0.0
				num := 0
				for i := startX; i < endX; i++ {
					i2 := i + oX
					for j := startY; j < endY; j++ {
						j2 := j + oY
						// pr1, pg1, pb1, _ := img1.At(i, j).RGBA()
						// pr2, pg2, pb2, _ := img2.At(i2, j2).RGBA()
						// dist += math.Abs(float64(pr1+pg1+pb1) - float64(pr2+pg2+pb2))
						_, p1, _, _ := img1.At(i, j).RGBA()
						_, p2, _, _ := img2.At(i2, j2).RGBA()
						dist += math.Abs(float64(p1) - float64(p2))
						num++
						// fmt.Printf("(%d,%d,%d,%d,%d,%d) -> %f\n", oX, oY, i, j, i2, j2, d)
					}
				}
				dist /= float64(num)
				mtx.Lock()
				if dist < minDist {
					minDist = dist
					minOffX = oX
					minOffY = oY
					// fmt.Printf("(%d,%d) -> (%f, %d)\n", oX, oY, dist, num)
				}
				mtx.Unlock()
			}
			ch <- struct{}{}
		}(ch, oXi)
		// Keep maximum number of threads
		nThreads++
		if nThreads == thrN {
			<-ch
			nThreads--
		}
	}
	for nThreads > 0 {
		<-ch
		nThreads--
	}
	return minOffX, minOffY, minDist
}

func alignImages(imageNames []string) error {
	// Threads
	thrsS := os.Getenv("N")
	thrs := -1
	if thrsS != "" {
		t, err := strconv.Atoi(thrsS)
		if err != nil {
			return err
		}
		thrs = t
	}
	thrN := thrs
	if thrs < 0 {
		thrN = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(thrN)
	fmt.Printf("Using %d (v)CPUs\n", thrN)

	// From X
	fromX := 0
	fromXS := os.Getenv("FROM_X")
	if fromXS != "" {
		v, err := strconv.Atoi(fromXS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("FROM_X must be from non-negative")
		}
		fromX = v
	}
	// From Y
	fromY := 0
	fromYS := os.Getenv("FROM_Y")
	if fromYS != "" {
		v, err := strconv.Atoi(fromYS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("FROM_Y must be from non-negative")
		}
		fromY = v
	}
	// Range X
	rangeX := 64
	rangeXS := os.Getenv("RANGE_X")
	if rangeXS != "" {
		v, err := strconv.Atoi(rangeXS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("RANGE_X must be range non-negative")
		}
		rangeX = v
	}
	// Range Y
	rangeY := 64
	rangeYS := os.Getenv("RANGE_Y")
	if rangeYS != "" {
		v, err := strconv.Atoi(rangeYS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("RANGE_Y must be range non-negative")
		}
		rangeY = v
	}
	// Size X
	sizeX := 200
	sizeXS := os.Getenv("SIZE_X")
	if sizeXS != "" {
		v, err := strconv.Atoi(sizeXS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("SIZE_X must be size non-negative")
		}
		sizeX = v
	}
	// Size Y
	sizeY := 200
	sizeYS := os.Getenv("SIZE_Y")
	if sizeYS != "" {
		v, err := strconv.Atoi(sizeYS)
		if err != nil {
			return err
		}
		if v < -1 {
			return fmt.Errorf("SIZE_Y must be size non-negative")
		}
		sizeY = v
	}

	// Flushing before endline
	flush := bufio.NewWriter(os.Stdout)

	ch := make(chan error)
	nThreads := 0
	var (
		x [3]int
		y [3]int
		m [3]image.Image
	)
	for i := 0; i < 3; i++ {
		go func(ch chan error, idx int) {
			// Input
			dtStartI := time.Now()
			reader, err := os.Open(imageNames[idx])
			if err != nil {
				ch <- err
				return
			}

			// Decode input
			var e error
			m[idx], _, e = image.Decode(reader)
			if e != nil {
				_ = reader.Close()
				ch <- e
				return
			}
			e = reader.Close()
			if e != nil {
				ch <- e
				return
			}
			bounds := m[idx].Bounds()
			x[idx] = bounds.Max.X
			y[idx] = bounds.Max.Y
			dtEndI := time.Now()
			fmt.Printf(" #%d: (%d x %d: %+v)...", idx, x[idx], y[idx], dtEndI.Sub(dtStartI))
			_ = flush.Flush()
			ch <- nil
		}(ch, i)
		// Keep maximum number of threads
		nThreads++
		if nThreads == thrN {
			e := <-ch
			if e != nil {
				return e
			}
			nThreads--
		}
	}
	for nThreads > 0 {
		e := <-ch
		if e != nil {
			return e
		}
		nThreads--
	}
	_ = flush.Flush()
	if fromX == 0 {
		fromX = x[0] / 2
	}
	if fromY == 0 {
		fromY = y[0] / 2
	}
	fmt.Printf(" middle: (%d,%d) range: (%d, %d), size: (%d, %d)...", fromX, fromY, rangeX, rangeY, sizeX, sizeY)
	if fromX-rangeX-sizeX < 0 {
		return fmt.Errorf("fromX-rangeX-sizeX=%d, it must be >= 0", fromX-rangeX-sizeX)
	}
	if fromY-rangeY-sizeY < 0 {
		return fmt.Errorf("fromY-rangeY-sizeY=%d, it must be >= 0", fromY-rangeY-sizeY)
	}
	minX := x[0]
	if x[1] < minX {
		minX = x[1]
	}
	if x[2] < minX {
		minX = x[2]
	}
	minY := y[0]
	if y[1] < minY {
		minY = y[1]
	}
	if y[2] < minY {
		minY = y[2]
	}
	if fromX+rangeX+sizeX >= minX {
		return fmt.Errorf("fromX+rangeX+sizeX=%d, it must be < %d", fromX+rangeX+sizeX, minX)
	}
	if fromY+rangeY+sizeY >= minY {
		return fmt.Errorf("fromY+rangeY+sizeY=%d, it must be < %d", fromY+rangeY+sizeY, minY)
	}

	// Now find minimums
	var (
		offX [3]int
		offY [3]int
		dist [3]float64
	)
	for idx := 0; idx < 3; idx++ {
		go func(ch chan error, i int) {
			// Input
			dtStartI := time.Now()
			j := i + 1
			if j == 3 {
				j = 0
			}
			offX[i], offY[i], dist[i] = optimizeAlignment(m[i], m[j], fromX, fromY, rangeX, rangeY, sizeX, sizeY, thrN)
			dtEndI := time.Now()
			fmt.Printf(" #%d<->#%d (in %v): offset: (%d, %d), dist: %f...", i, j, dtEndI.Sub(dtStartI), offX[i], offY[i], dist[i])
			_ = flush.Flush()
			if offX[i] == -rangeX {
				fmt.Printf("\nWARNING: aligning #%d image to #%d image required maximum %d shift left of the 2nd image, probably unaligned, try to increase RANGE_X and SIZE_X\n", i, j, rangeX)
			}
			if offX[i] == rangeX {
				fmt.Printf("\nWARNING: aligning #%d image to #%d image required maximum %d shift right of the 2nd image, probably unaligned, try to increase RANGE_X and SIZE_X\n", i, j, rangeX)
			}
			if offY[i] == -rangeY {
				fmt.Printf("\nWARNING: aligning #%d image to #%d image required maximum %d shift up of the 2nd image, probably unaligned, try to increase RANGE_Y and SIZE_Y\n", i, j, rangeY)
			}
			if offY[i] == rangeY {
				fmt.Printf("\nWARNING: aligning #%d image to #%d image required maximum %d shift down of the 2nd image, probably unaligned, try to increase RANGE_Y and SIZE_Y\n", i, j, rangeY)
			}
			ch <- nil
		}(ch, idx)
		// Keep maximum number of threads
		nThreads++
		if nThreads == thrN {
			e := <-ch
			if e != nil {
				return e
			}
			nThreads--
		}
	}
	for nThreads > 0 {
		e := <-ch
		if e != nil {
			return e
		}
		nThreads--
	}
	_ = flush.Flush()
	// Now figure out order of alignment and final image size
	minI := 0
	if dist[1] < dist[minI] {
		minI = 1
	}
	if dist[2] < dist[minI] {
		minI = 2
	}
	var oX, oY [3]int
	switch minI {
	case 0:
		// 0->0, 0->1, 0->2
		oX = [3]int{0, offX[0], -offX[2]}
		oY = [3]int{0, offY[0], -offY[2]}
	case 1:
		// 1->0, 1->1, 1->2
		oX = [3]int{-offX[0], 0, offX[1]}
		oY = [3]int{-offY[0], 0, offY[1]}
	case 2:
		// 2->0, 2->1, 2->2
		oX = [3]int{offX[2], -offX[1], 0}
		oY = [3]int{offY[2], -offY[1], 0}
	}
	target := image.NewRGBA(image.Rect(0, 0, minX, minY))
	var (
		ii [3]int
		jj [3]int
	)
	for i := 0; i < minX; i++ {
		for k := 0; k < 3; k++ {
			ii[k] = i + oX[k]
			if ii[k] < 0 {
				ii[k] = 0
			}
			if ii[k] >= minX {
				ii[k] = minX - 1
			}
		}
		for j := 0; j < minY; j++ {
			for k := 0; k < 3; k++ {
				jj[k] = j + oY[k]
				if jj[k] < 0 {
					jj[k] = 0
				}
				if jj[k] >= minY {
					jj[k] = minY - 1
				}
			}
			_, pr, _, _ := m[0].At(ii[0], jj[0]).RGBA()
			_, pg, _, _ := m[1].At(ii[1], jj[1]).RGBA()
			_, pb, _, _ := m[2].At(ii[2], jj[2]).RGBA()
			pixel := color.RGBA64{uint16(pr), uint16(pg), uint16(pb), 0}
			target.Set(i, j, pixel)
		}
	}
	lfn := imageNames[3]
	fi, err := os.Create(lfn)
	if err != nil {
		return err
	}
	dtStart := time.Now()
	if strings.Contains(lfn, ".png") {
		enc := png.Encoder{CompressionLevel: 9}
		err = enc.Encode(fi, target)
	} else if strings.Contains(lfn, ".jpg") || strings.Contains(lfn, ".jpeg") {
		err = jpeg.Encode(fi, target, &jpeg.Options{Quality: 90})
	} else if strings.Contains(lfn, ".gif") {
		err = gif.Encode(fi, target, nil)
	} else if strings.Contains(lfn, ".tif") {
		err = tiff.Encode(fi, target, nil)
	} else if strings.Contains(lfn, ".bmp") {
		err = bmp.Encode(fi, target)
	}
	if err != nil {
		_ = fi.Close()
		return err
	}
	err = fi.Close()
	if err != nil {
		return err
	}
	dtEnd := time.Now()
	fmt.Printf("%s written in %v\n", lfn, dtEnd.Sub(dtStart))
	return nil
}

func main() {
	dtStart := time.Now()
	if len(os.Args) >= 5 {
		err := alignImages(os.Args[1:])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	} else {
		fmt.Printf("Please provide 3 iamges for R, G, B channel to align and 1 image for output")
		helpStr := `
Environment variables:
N - how many (v)CPUs use, defaults to autodetect
FROM_X - where is the x value for image circle to start aligning from (middle of the first image if not specified)
FROM_Y - where is the y value for image circle to start aligning from (middle of the first image if not specified)
RANGE_X - how many x pixels check around start x (defaults to 64, which gives 64+64+1 = 129 checks)
RANGE_Y - how many y pixels check around start y (defaults to 64, which gives 64+64+1 = 129 checks)
SIZE_X - how many x pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels)
SIZE_Y - how many y pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels)
`
		fmt.Printf("%s\n", helpStr)
	}
	dtEnd := time.Now()
	fmt.Printf("Time: %v\n", dtEnd.Sub(dtStart))
}
