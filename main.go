//usr/bin/env go run $0 $@; exit $?
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/djherbis/times"

	"github.com/ivanzoid/copyNewPhotos/config"
)

var (
	flagFolderCount int
	flagList        bool
)

type Dir struct {
	Path             string
	Name             string
	CreateTimeString string
	CreateTime       time.Time
}

type Dirs []Dir

func init() {
	flag.IntVar(&flagFolderCount, "c", 1, "Count of folders to copy")
	flag.BoolVar(&flagList, "ls", false, "Use to list directories")
}

func main() {

	flag.Parse()

	cfg, _ := config.Load()

	if cfg == nil || len(cfg.PhotoPath) == 0 {
		log.Fatal("Please add photoPath to .copyNewPhotos/config.json")
	}

	mountPoints, err := exfatMountPoints()
	if err != nil {
		log.Fatal(err)
	}

	year := time.Now().Format("2006")
	photosDir := path.Join(cfg.PhotoPath, year)

	localPhotoDirs := localPhotoDirs(photosDir)

	for _, mountPoint := range mountPoints {
		dcimPath := path.Join(mountPoint, "DCIM")
		exists, err := exists(dcimPath)
		if err != nil || !exists {
			continue
		}

		flashDirs := subfolders(dcimPath)

		if flagList {
			listFlashDirs(flashDirs)
		} else {
			copyFlashDirs(flashDirs, localPhotoDirs, photosDir)
		}

	}
}
func copyFlashDirs(flashDirs Dirs, localPhotoDirs Dirs, photosDir string) {
	count := 0
	for i := len(flashDirs) - 1; i >= 0; i-- {
		if count >= flagFolderCount {
			break
		}
		flashDir := flashDirs[i]
		if !localPhotoDirs.hasSameLocalDir(flashDir) {
			copyPhotoDirToDir(flashDir, photosDir)
			count++
		} else {
			fmt.Printf("%s: already copied\n", flashDir.Name)
		}
	}
}
func listFlashDirs(dirs Dirs) {
	for _, dir := range dirs {
		fmt.Printf("file://%v (%v)\n", dir.Path, dir.CreateTime)
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func exfatMountPoints() ([]string, error) {
	mountOutAsBytes, err := exec.Command("mount", "-t", "exfat").Output()
	if err != nil {
		return nil, err
	}

	mountOutAsString := string(mountOutAsBytes)
	mountStrings := strings.Split(mountOutAsString, "\n")

	mountPoints := make([]string, 0)

	for _, mountString := range mountStrings {
		mountString = strings.TrimSpace(mountString)
		if len(mountString) == 0 {
			continue
		}

		comps := strings.Split(mountString, " ")

		if len(comps) < 3 {
			fmt.Fprintf(os.Stderr, "Can't parse mount string: \"%s\"\n", mountString)
			continue
		}
		mountPoints = append(mountPoints, comps[2])
	}

	return mountPoints, nil
}

func subfolders(folder string) Dirs {
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't read dir \"%s\": %v\n", folder, err)
		return nil
	}

	dirs := make([]Dir, 0)

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		var dir Dir
		dir.Path = path.Join(folder, file.Name())

		t, err := times.Stat(dir.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't stat dir \"%s\": %v\n", dir.Path, err)
			continue
		}
		dir.CreateTime = t.BirthTime()
		dir.CreateTimeString = t.BirthTime().Format("01-02")
		dir.Name = file.Name()

		dirs = append(dirs, dir)
	}

	return dirs
}

func localPhotoDirs(localPhotosRootDir string) Dirs {

	subfolders := subfolders(localPhotosRootDir)

	for index := range subfolders {
		subfolder := &subfolders[index]
		if len(subfolder.Name) < 8 {
			continue
		}
		subfolder.CreateTimeString = subfolder.Name[:8]
	}

	return subfolders
}

func (dirs Dirs) hasSameLocalDir(flashDir Dir) bool {
	for _, localDir := range dirs {
		if localDir.CreateTimeString == flashDir.CreateTimeString {
			return true
		}
	}
	return false
}

func copyPhotoDirToDir(src Dir, dstParent string) {
	files, err := ioutil.ReadDir(src.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't read dir \"%s\": %v\n", src.Path, err)
		return
	}

	localDirName := src.CreateTimeString
	localDirPath := path.Join(dstParent, localDirName)

	err = os.Mkdir(localDirPath, 0777)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't create dir \"%s\": %v\n", localDirPath, err)
		return
	}

	if len(files) == 0 {
		return
	}

	fmt.Printf("%s -> %s:\n", src.Name, localDirName)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		srcPath := path.Join(src.Path, file.Name())
		dstPath := path.Join(localDirPath, file.Name())

		fmt.Printf("\t%s\n", file.Name())

		copyFile(srcPath, dstPath)
	}
}

func copyFile(src string, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}

	return d.Close()
}
