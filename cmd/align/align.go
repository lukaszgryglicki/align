package main

import (
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

const (
	extJPG  = ".jpg"
	extJPEG = ".jpeg"
	extPNG  = ".png"
	extTIF  = ".tif"
	extGIF  = ".git"
	extBMP  = ".bmp"
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
	fmt.Printf("Using %d (v)CPU(s)\n", thrN)

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

	// 8bit mode
	bits8S := os.Getenv("BITS8")
	bits8 := bits8S != ""
	if bits8 {
		fmt.Printf("Using 8bit output file\n")
	}

	// Pixel values shift
	pxvShift := 0
	pxvShiftS := os.Getenv("PXV_SHIFT")
	if pxvShiftS != "" {
		v, err := strconv.Atoi(pxvShiftS)
		if err != nil {
			return err
		}
		if v < -31 || v > 31 {
			return fmt.Errorf("PXV_SHIFT must be from [-31, 31]")
		}
		pxvShift = v
	}
	if pxvShift != 0 {
		fmt.Printf("Pixel values shift: %d\n", pxvShift)
	}

	// JPEG Quality
	jpegqStr := os.Getenv("Q")
	jpegq := -1
	if jpegqStr != "" {
		v, err := strconv.Atoi(jpegqStr)
		if err != nil {
			return err
		}
		if v < 1 || v > 100 {
			return fmt.Errorf("Q must be from 1-100 range")
		}
		jpegq = v
		fmt.Printf("JPEG quality set to: %d%%\n", jpegq)
	}

	// PNG Quality
	pngqStr := os.Getenv("PQ")
	pngq := png.DefaultCompression
	if pngqStr != "" {
		v, err := strconv.Atoi(pngqStr)
		if err != nil {
			return err
		}
		if v < 0 || v > 3 {
			return fmt.Errorf("PQ must be from 0-3 range")
		}
		pngq = png.CompressionLevel(-v)
		fmt.Printf("PNG quality set to: #%d\n", -pngq)
	}

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
			fmt.Printf("#%d: (%d x %d: %+v)\n", idx, x[idx], y[idx], dtEndI.Sub(dtStartI))
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
	if fromX == 0 {
		fromX = x[0] / 2
	}
	if fromY == 0 {
		fromY = y[0] / 2
	}
	fmt.Printf("Middle: (%d,%d) range: (%d, %d), size: (%d, %d)\n", fromX, fromY, rangeX, rangeY, sizeX, sizeY)
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
			j := i + 1
			if j == 3 {
				j = 0
			}
			got := false
			str := os.Getenv(fmt.Sprintf("HINT_%d%d", i, j))
			if str != "" {
				n, err := fmt.Sscanf(str, "%d,%d,%f", &offX[i], &offY[i], &dist[i])
				if err == nil && n == 3 {
					got = true
				}
			}
			dtStartI := time.Now()
			if !got {
				offX[i], offY[i], dist[i] = optimizeAlignment(m[i], m[j], fromX, fromY, rangeX, rangeY, sizeX, sizeY, thrN)
			}
			dtEndI := time.Now()
			fmt.Printf("#%d<->#%d (in %v): offset: (%d, %d), dist: %f\n", i, j, dtEndI.Sub(dtStartI), offX[i], offY[i], dist[i])
			if offX[i] == -rangeX {
				fmt.Printf("WARNING: aligning #%d image to #%d image required maximum %d shift left of the 2nd image, probably unaligned, try to increase RANGE_X and SIZE_X\n", i, j, rangeX)
			}
			if offX[i] == rangeX {
				fmt.Printf("WARNING: aligning #%d image to #%d image required maximum %d shift right of the 2nd image, probably unaligned, try to increase RANGE_X and SIZE_X\n", i, j, rangeX)
			}
			if offY[i] == -rangeY {
				fmt.Printf("WARNING: aligning #%d image to #%d image required maximum %d shift up of the 2nd image, probably unaligned, try to increase RANGE_Y and SIZE_Y\n", i, j, rangeY)
			}
			if offY[i] == rangeY {
				fmt.Printf("WARNING: aligning #%d image to #%d image required maximum %d shift down of the 2nd image, probably unaligned, try to increase RANGE_Y and SIZE_Y\n", i, j, rangeY)
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
		fmt.Printf("Base align 0->1 at distance: %f\n", dist[0])
		// 0->0, 0->1, 0->2
		oX = [3]int{0, offX[0], -offX[2]}
		oY = [3]int{0, offY[0], -offY[2]}
	case 1:
		fmt.Printf("Base align 1->2 at distance: %f\n", dist[1])
		// 1->0, 1->1, 1->2
		oX = [3]int{-offX[0], 0, offX[1]}
		oY = [3]int{-offY[0], 0, offY[1]}
	case 2:
		fmt.Printf("Base align 2->0 at distance: %f\n", dist[2])
		// 2->0, 2->1, 2->2
		oX = [3]int{offX[2], -offX[1], 0}
		oY = [3]int{offY[2], -offY[1], 0}
	}
	fmt.Printf("Offsets: (%+v, %+v)\n", oX, oY)
	fmt.Printf("Generating output image data...\n")
	dtStart := time.Now()
	var (
		target16 *image.RGBA64
		target8  *image.RGBA
	)
	if bits8 {
		target8 = image.NewRGBA(image.Rect(0, 0, minX, minY))
	} else {
		target16 = image.NewRGBA64(image.Rect(0, 0, minX, minY))
	}
	var (
		ii  [3]int
		jj  [3]int
		val float64
		n   int64
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
			if pxvShift > 0 {
				pr >>= pxvShift
				pg >>= pxvShift
				pb >>= pxvShift
			} else if pxvShift < 0 {
				pr <<= -pxvShift
				pg <<= -pxvShift
				pb <<= -pxvShift
			}
			val += float64(pr + pg + pb)
			n++
			if bits8 {
				pixel := color.RGBA{uint8(pr), uint8(pg), uint8(pb), 0xff}
				target8.Set(i, j, pixel)
			} else {
				pixel := color.RGBA64{uint16(pr), uint16(pg), uint16(pb), 0xffff}
				target16.Set(i, j, pixel)
			}
		}
	}
	val /= float64(n * 3)
	fmt.Printf("AVG pixel value: %f\n", val)
	dtEnd := time.Now()
	fmt.Printf("Generated in %+v\n", dtEnd.Sub(dtStart))
	fmt.Printf("Saving image...\n")
	fn := imageNames[3]
	lfn := strings.ToLower(fn)
	fi, err := os.Create(fn)
	if err != nil {
		return err
	}
	dtStart = time.Now()
	if bits8 {
		if strings.Contains(lfn, extPNG) {
			fmt.Printf("Using 8bit PNG output\n")
			enc := png.Encoder{CompressionLevel: pngq}
			err = enc.Encode(fi, target8)
		} else if strings.Contains(lfn, extJPG) || strings.Contains(lfn, extJPEG) {
			fmt.Printf("Using 8bit JPG output\n")
			var jopts *jpeg.Options
			if jpegq >= 0 {
				jopts = &jpeg.Options{Quality: jpegq}
			}
			err = jpeg.Encode(fi, target8, jopts)
		} else if strings.Contains(lfn, extGIF) {
			fmt.Printf("Using 8bit GIF output\n")
			err = gif.Encode(fi, target8, nil)
		} else if strings.Contains(lfn, extTIF) {
			fmt.Printf("Using 8bit TIFF output\n")
			err = tiff.Encode(fi, target8, nil)
		} else if strings.Contains(lfn, extBMP) {
			fmt.Printf("Using 8bit BMP output\n")
			err = bmp.Encode(fi, target8)
		}
	} else {
		if strings.Contains(lfn, extPNG) {
			fmt.Printf("Using 16bit PNG output\n")
			enc := png.Encoder{CompressionLevel: pngq}
			err = enc.Encode(fi, target16)
		} else if strings.Contains(lfn, extJPG) || strings.Contains(lfn, extJPEG) {
			fmt.Printf("Using 16bit JPG output\n")
			var jopts *jpeg.Options
			if jpegq >= 0 {
				jopts = &jpeg.Options{Quality: jpegq}
			}
			err = jpeg.Encode(fi, target16, jopts)
		} else if strings.Contains(lfn, extGIF) {
			fmt.Printf("Using 16bit GIF output\n")
			err = gif.Encode(fi, target16, nil)
		} else if strings.Contains(lfn, extTIF) {
			fmt.Printf("Using 16bit TIF output\n")
			err = tiff.Encode(fi, target16, nil)
		} else if strings.Contains(lfn, extBMP) {
			fmt.Printf("Using 16bit BMP output\n")
			err = bmp.Encode(fi, target16)
		}
	}
	if err != nil {
		_ = fi.Close()
		return err
	}
	err = fi.Close()
	if err != nil {
		return err
	}
	dtEnd = time.Now()
	fmt.Printf("%s written in %v\n", fn, dtEnd.Sub(dtStart))
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
Supported input/output formats: JPG, PNG, TIF, GIF, BMP.
Environment variables:
N - how many (v)CPUs use, defaults to autodetect
BITS8 - set 8 bit mode (default is 16 bit)
FROM_X - where is the x value for image circle to start aligning from (middle of the first image if not specified)
FROM_Y - where is the y value for image circle to start aligning from (middle of the first image if not specified)
RANGE_X - how many x pixels check around start x (defaults to 64, which gives 64+64+1 = 129 checks)
RANGE_Y - how many y pixels check around start y (defaults to 64, which gives 64+64+1 = 129 checks)
SIZE_X - how many x pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels)
SIZE_Y - how many y pixels check in single pass (defaults to 200, which gives 200+200+1 = 401x401 = 160801 pixels)
HINT_01/HINT_12/HINT_20 - provide a hint so no optimization is needed, example: -2,-2,7855.34
PXV_SHIFT - shift ouput pixel values right by this amount, can also be negative
Q - jpeg quality 1-100, will use library default if not specified
PQ - png quality 0-3 (0 is default): 0=DefaultCompression, 1=NoCompression, 2=BestSpeed, 3=BestCompression
`
		fmt.Printf("%s\n", helpStr)
	}
	dtEnd := time.Now()
	fmt.Printf("Time: %v\n", dtEnd.Sub(dtStart))
}
