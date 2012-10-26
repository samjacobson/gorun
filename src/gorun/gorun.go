package main

import (
	"os"
	"log"
	"os/exec"
	"fmt"
	"regexp"
	"strings"
	"strconv"
	"path"
	"bufio"
)

func goCommentLines(in, out string, lines[]int) (err error) {
	var inf, outf *os.File
	if inf, err = os.Open(in); err != nil {
		return err
	}
	defer inf.Close()
	if outf, err = os.Create(out); err != nil {
		return err
	}
	defer outf.Close()

	binf := bufio.NewReader(inf)
	lineno := 0
	idx := 0
	var line string
	for line, err = binf.ReadString('\n'); err == nil; line, err = binf.ReadString('\n') {
		lineno++
		if idx < len(lines) && lineno == lines[idx] {
			idx++
			line = "//" + line
		}
		_, err = outf.Write([]byte(line))
		if err != nil {
			return
		}
	}
	_, err = outf.Write([]byte(line))

	return
}

func goFix(infiles []string, errors string) (gofiles, tmps[]string, canFix bool, err error) {
	working := make(map[string]bool)
	for _, inf := range infiles {
		working[inf] = true
	}

	reImportError, _ := regexp.Compile(`^([^:]+):(\d+): imported and not used: "([^"]+)"$`)
	toFix := make(map[string][]int)
	for _, line := range strings.Split(errors, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ie := reImportError.FindStringSubmatch(strings.TrimRight(line, "\n"))
		if len(ie) == 0 {
			// There's an error that's not a comment or import error; we therefore can't fix it
			canFix = false
			return
		}
		line, _ := strconv.Atoi(ie[2])
		toFix[ie[1]] = append(toFix[ie[1]], line)
	}
	canFix = true

	for fname, badlines := range toFix {
		inf := path.Clean(fname)
		delete(working, inf)
		tmpf := path.Join(path.Dir(inf), "tmp." + path.Base(inf))
		if err = goCommentLines(inf, tmpf, badlines); err != nil {
			return
		}
		tmps = append(tmps, tmpf)
		working[tmpf] = true
	}
	for k, _ := range working {
		gofiles = append(gofiles, k)
	}

	return
}

func goRun(gofiles []string) {
	args := append([]string{"run"}, gofiles...)
	run := exec.Command("go", args...)
	run.Stdout = os.Stdout
	run.Stderr = os.Stderr
	run.Stdin = os.Stdin
	run.Run()
}

func goBuild(gofiles []string) string {
	args := append([]string{"build"}, gofiles...)
	build := exec.Command("go", args...)
	stdout, err := build.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return string(stdout)
		}
		log.Fatal(err)
	}

	return ""
}

func remove(files []string) {
	for _, pth := range files {
		os.Remove(pth)
	}
}

func main() {
	var gofiles []string
	for _, fname := range os.Args[1:] {		// must be 1: when running as an executable, 2: when running with "go run", ie "go run gorun.go -- badimport.go..."
		gofiles = append(gofiles, path.Clean(fname))
	}
	errors := goBuild(gofiles)
	if errors == "" {
//		fmt.Print("Running no cleaning required\n")
		goRun(gofiles)
	} else {
		gofiles, tmps, canFix, err := goFix(gofiles, errors)
		if err != nil {
			log.Fatal(err)
		} else if ! canFix {
			fmt.Print(errors)
			return
		}
//		fmt.Print("Running after cleaning\n")
		goRun(gofiles)
		remove(tmps)
	}
}

