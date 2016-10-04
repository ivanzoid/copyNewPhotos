//usr/bin/env go run $0 $@; exit $?
package main

import (
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
)

type Dir struct {
	Path       string
	Name       string
	CreateTime string
}

type Dirs []Dir

func main() {

	mountPoints, err := exfatMountPoints()
	if err != nil {
		log.Fatal(err)
	}

	localPhotosRootDir := localPhotosRootDir()
	localPhotoDirs := localPhotoDirs(localPhotosRootDir)

	for _, mountPoint := range mountPoints {
		dcimPath := path.Join(mountPoint, "DCIM")
		exists, err := exists(dcimPath)
		if err != nil || !exists {
			continue
		}

		flashDirs := subfolders(dcimPath)

		for _, flashDir := range flashDirs {
			if !localPhotoDirs.hasSameLocalDir(flashDir) {
				copyPhotoDirToDir(flashDir, localPhotosRootDir)
			} else {
				fmt.Printf("%s: already copied\n", flashDir.Name)
			}
		}
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
		dir.CreateTime = t.BirthTime().Format("06-01-02")
		dir.Name = file.Name()

		dirs = append(dirs, dir)
	}

	return dirs
}

func localPhotosRootDir() string {
	year := time.Now().Format("2006")
	photosRootDir := path.Join("/Volumes/BigData/Photo/", year)
	return photosRootDir
}

func localPhotoDirs(localPhotosRootDir string) Dirs {
	subfolders := subfolders(localPhotosRootDir)

	for index := range subfolders {
		subfolder := &subfolders[index]
		if len(subfolder.Name) < 8 {
			continue
		}
		subfolder.CreateTime = subfolder.Name[:8]
	}

	return subfolders
}

func (dirs Dirs) hasSameLocalDir(flashDir Dir) bool {
	for _, localDir := range dirs {
		if localDir.CreateTime == flashDir.CreateTime {
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

	localDirName := src.CreateTime
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
