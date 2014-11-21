package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

func readFile(path string) []string {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	return strings.Split(string(dat), "\n")
}

func writeFile(path string, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

func bumpVersion(version string) string {
	prev, _ := strconv.Atoi(strings.Replace(version, "v", "", 1))
	return "v" + strconv.Itoa(prev+1)
}

func run(cmd string) string {
	args := strings.Split(cmd, " ")
	cmd, args = args[0], args[1:]

	cmdOutput := &bytes.Buffer{}

	proc := exec.Command(cmd, args...)
	proc.Stdout = cmdOutput
	proc.Stderr = cmdOutput

	if err := proc.Run(); err != nil {
		log.Fatalln(err)
	}

	out := string(cmdOutput.Bytes())
	if out != "" {
		fmt.Println(out)
	}

	return out
}

func main() {
	version := bumpVersion(run("git describe --abbrev=0 --tags"))
	run("git add .")
	run("git commit -m " + version)
	run("git tag " + version)
	run("git push origin master --tags")
	run("git push -f origin master:production")
}
