package main

import (
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/esimov/stackblur-go"
	"github.com/fsnotify/fsnotify"
)

func main() {
	var path string
	if len(os.Args) < 2 {
		log.Fatal("No path provided")
	}

	path = os.Args[1]
	log.Println("path", path)
	if i, err := os.Stat(path); errors.Is(err, os.ErrNotExist) || i.IsDir() {
		log.Fatal("File does not exist or is a directory")
	}

	log.Println("Running...")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	var lastWrite time.Time

	go func() {
		for {
			select {
			case _, ok := <-watcher.Events:
				if !ok {
					return
				} else if lastWrite.Add(8 * time.Second).After(time.Now()) {
					log.Println("skipping")
				} else {
					lastWrite = time.Now()
					blur(path)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
	}

	// Block main goroutine forever.
	<-make(chan struct{})
}

func blur(path string) {
	f, err := os.ReadFile(path)
	must(err)

	file := string(f)
	must(err)

	var blurEnabled bool
	var blurRadius uint32
	var backgroundImage string

	lines := strings.Split(file, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "\"blurEnable\"") && strings.Contains(line, "true") {
			blurEnabled = true
		} else if strings.HasPrefix(strings.TrimSpace(line), "\"blurRadius\"") {
			parsedRadius, err := parseBlurRadius(line)
			must(err)
			blurRadius = parsedRadius
		} else if strings.HasPrefix(strings.TrimSpace(line), "\"backgroundImage\"") {
			backgroundImage = parseBackgroundImage(line)
		}
	}

	log.Println("blurEnabled", blurEnabled)
	log.Println("blurRadius", blurRadius)
	log.Println("backgroundImage", backgroundImage)

	if !blurEnabled {
		log.Println(len(lines))
		log.Println(len(file))
		log.Println("Blur is disabled")
		return
	} else if blurRadius == 0 {
		log.Println("Blur radius is 0")
		return
	} else if backgroundImage == "" {
		log.Println("No background image set")
		return
	} else if strings.Contains(backgroundImage, "blurred-") {
		log.Println("Background image is already blurred")
		return
	}

	img, err := getImageFromFilePath(backgroundImage)
	must(err)

	img, err = stackblur.Process(img, blurRadius)
	must(err)

	// Determine the format of the original image
	format := backgroundImage[strings.LastIndex(backgroundImage, ".")+1:]
	if format == "" {
		log.Fatal("Could not determine image format")
	}

	log.Println("format", format)

	var fileName string
	if strings.Contains(backgroundImage, "/") {
		fileName = strings.Split(backgroundImage, "/")[len(strings.Split(backgroundImage, "/"))-1]
	} else if strings.Contains(backgroundImage, "\\") {
		fileName = strings.Split(backgroundImage, "\\")[len(strings.Split(backgroundImage, "\\"))-1]
	} else {
		fileName = backgroundImage
	}

	realFileName := fileName
	fileName = fmt.Sprintf("blurred-r%d-%s", blurRadius, fileName)
	log.Println("fileName", fileName)

	outputFile, err := os.Create(os.TempDir() + "/" + fileName)
	must(err)

	log.Println("outputFile", outputFile.Name())
	defer outputFile.Close()

	switch strings.ToUpper(format) {
	case "JPEG":
		err = jpeg.Encode(outputFile, img, nil)
	case "JPG":
		err = jpeg.Encode(outputFile, img, nil)
	case "PNG":
		err = png.Encode(outputFile, img)
	case "GIF":
		err = gif.Encode(outputFile, img, nil)
	default:
		log.Fatal(errors.New("Unsupported image format"))
	}

	content := []string{}
	for _, line := range lines {
		if strings.Contains(line, realFileName) {
			str := strings.ReplaceAll(os.TempDir(), "\\", "/")
			space := strings.Repeat(" ", strings.Index(line, "\""))

			line = strings.TrimSpace(line)
			// content = append(content, space+"// "+line)
			content = append(content, space+line)
			content = append(content, space+"\"backgroundImage\": \""+str+"/"+fileName+"\",")
			continue
		}

		content = append(content, line)
	}

	os.WriteFile(path, []byte(strings.Join(content, "\n")), 0644)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getImageFromFilePath(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	return image, err
}

func parseBlurRadius(line string) (uint32, error) {
	lastKeyQuote := strings.Index(line, "\":")
	line = line[lastKeyQuote+2:]
	lastValueQuote := strings.Index(line, ",")
	valueStr := strings.TrimSpace(line[:lastValueQuote])

	log.Println(valueStr)

	value, err := strconv.Atoi(valueStr)

	if err != nil {
		return 0, err
	}

	return uint32(value), nil
}

func parseBackgroundImage(line string) string {
	lastKeyQuote := strings.Index(line, "\":")
	line = line[lastKeyQuote+2:]
	lastValueQuote := strings.Index(line, ",")
	valueStr := strings.TrimSpace(line[:lastValueQuote])
	valueStr = valueStr[1 : len(valueStr)-1]
	log.Println(valueStr)
	return valueStr
}
