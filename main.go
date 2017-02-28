package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// https://stackoverflow.com/a/12527546
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// https://stackoverflow.com/a/24792688
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func readMapFile(filename string) map[string]string {
	filemap := make(map[string]string)
	file, err := os.Open(filename)
	if err != nil {
		log.Println(err)
		return filemap
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rule := strings.Split(scanner.Text(), "=")
		filemap[rule[0]] = rule[1]
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return filemap
	}

	return filemap
}

func readTdesktopFile(filefolder string) map[string]string {
	filemap := make(map[string]string)
	file, err := os.Open(filefolder + "/colors.tdesktop-theme")
	if err != nil {
		log.Fatal(err)
		return filemap
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		themeline := scanner.Text()
		rule := strings.Split(themeline, ": ")
		if len(rule) >= 2 {
			rulekey := rule[0]
			rulevalue := strings.Split(rule[1], ";")[0]
			if !strings.HasPrefix(rulevalue, "#") {
				rulevalue = filemap[rulevalue]
			}
			rulevalue = strings.TrimPrefix(rulevalue, "#")
			filemap[rulekey] = rulevalue
		}
	}
	filemap["whatever"] = "ff00ff"
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
		return filemap
	}

	return filemap
}

func paramsToFilename(isTiled bool, isJpg bool) string {
	var name string
	var ext string
	if isTiled {
		name = "tiled"
	} else {
		name = "background"
	}

	if isJpg {
		ext = ".jpg"
	} else {
		ext = ".png"
	}

	return name + ext
}

func convertBg(filename string) {
	var istiled bool
	var isjpg bool
	var fname string
	if fileExists(filename + "/tiled.jpg") {
		istiled = true
		isjpg = true
	} else if fileExists(filename + "/tiled.png") {
		istiled = true
		isjpg = false
	} else if fileExists(filename + "/background.jpg") {
		istiled = false
		isjpg = true
	} else if fileExists(filename + "/background.png") {
		istiled = false
		isjpg = false
	} else {
		log.Println("Bg not found")
	}

	fname = filename + "/" + paramsToFilename(istiled, isjpg)

	var img image.Image

	if isjpg {
		fimg, _ := os.Open(fname)
		defer fimg.Close()
		img, _ = jpeg.Decode(fimg)
	} else {
		fimg, _ := os.Open(fname)
		defer fimg.Close()
		img, _ = png.Decode(fimg)
	}

	imgsizex := 1920
	imgsizey := 1920

	back := image.NewRGBA(image.Rect(0, 0, imgsizex, imgsizey))

	if istiled {
		tilesizex := img.Bounds().Max.X
		tilesizey := img.Bounds().Max.Y
		stepx := imgsizex / tilesizex
		stepy := imgsizey / tilesizey
		for x := 0; x <= stepx; x++ {
			for y := 0; y <= stepy; y++ {
				draw.Draw(back, back.Bounds(), img, image.Point{-x * tilesizex, -y * tilesizey}, draw.Src)
			}
		}
	}

	toimg, _ := os.Create(filename + "/converted.jpg")
	defer toimg.Close()
	if istiled {
		err := jpeg.Encode(toimg, back, &jpeg.Options{90})
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err := jpeg.Encode(toimg, img, &jpeg.Options{90})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func makeAttheme(mapfile, tdesktopmap, transmap, overridemap map[string]string, filename string) {
	var color string
	atthememap := make(map[string]string)
	file, err := os.OpenFile(
		"./atthemes/"+filename+".attheme",
		os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
		0666,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	for key, value := range mapfile {
		var desktopvalue string
		if overridemap[key] != "" {
			desktopvalue = overridemap[key]
			delete(overridemap, key)
		} else {
			desktopvalue = tdesktopmap[value]
		}
		if len(desktopvalue) == 6 {
			if transmap[key] != "" {
				color = transmap[key] + desktopvalue
			} else {
				color = desktopvalue
			}
		} else if len(desktopvalue) == 8 {
			color = desktopvalue[6:] + desktopvalue[:6]
		} else {
			fmt.Println("Key " + mapfile[key] + " missing from .tdesktop-theme")
			color = "00ff00"
		}
		atthememap[key] = strings.ToUpper(color)
	}
	if len(overridemap) != 0 {
		for key, value := range overridemap {
			trans := "ff"
			if len(value) == 6 {
				if transmap[key] != "" {
					color = transmap[key] + value
				} else {
					color = trans + value
				}
			} else {
				color = value[6:] + value[:6]
			}
			atthememap[key] = color
		}
	}
	for key, value := range atthememap {
		byteSlice := []byte(key + "=#" + value + "\n")
		_, err := file.Write(byteSlice)
		if err != nil {
			log.Fatal(err)
		}
	}
	if atthememap["chat_wallpaper"] == "" {
		fi, err := os.Open("./wip/" + filename + "/converted.jpg")
		if err != nil {
			panic(err)
		}
		_, err = file.Write([]byte("WPS\n"))
		if err != nil {
			log.Fatal(err)
		}

		buf := make([]byte, 1024)
		for {
			n, err := fi.Read(buf)
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			if n == 0 {
				break
			}
			if _, err := file.Write(buf[:n]); err != nil {
				log.Fatal(err)
			}
		}

		_, err = file.Write([]byte("\nWPE"))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func printFooter() {
	text := `
Converting done.
If you have any bugs feel free to contact me:
https://t.me/TDeskDroid
https://github.com/MrYadro/goTDeskDroid/issues/new`
	fmt.Println(text)
}

func readOverrideFile(filename string) map[string]string {
	overridemap := make(map[string]string)
	fullname := ""
	for _, name := range strings.Split(filename, ".") {
		fullname += name + "."
		fmt.Println("Looking for overrides at " + fullname + "map")
		for key, value := range readMapFile(fullname + "map") {
			overridemap[key] = value
		}
	}
	return overridemap
}

func prepareFolders() {
	os.Mkdir("./wip", 0777)
	os.Mkdir("./atthemes", 0777)
}

func main() {
	prepareFolders()
	files, _ := ioutil.ReadDir("./")
	for _, f := range files {
		filename := f.Name()
		filefolder := strings.TrimSuffix(filename, ".tdesktop-theme")
		mapfile := readMapFile("theme.map")
		transmapfile := readMapFile("trans.map")
		if strings.HasSuffix(filename, "tdesktop-theme") {
			fmt.Println("\nConverting", filefolder)
			unzip(filename, "./wip/"+filefolder)
			tdesktopmapfile := readTdesktopFile("./wip/" + filefolder)
			overridemapfile := readOverrideFile(filefolder)
			convertBg("./wip/" + filefolder)
			makeAttheme(mapfile, tdesktopmapfile, transmapfile, overridemapfile, filefolder)
		}
	}
	printFooter()
}
