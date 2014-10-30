package main

import (
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"strings"
	"os"
	"os/exec"
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

func bumpVersion(path string) (version string) {
	lines := readFile(path)
	re := regexp.MustCompile("^version:")

	for i, line := range lines {
		if re.FindStringIndex(line) != nil {
			prev, _ := strconv.Atoi(strings.Replace(line, "version: v", "", 1))
			version = "v" + strconv.Itoa(prev + 1)
			lines[i] = "version: " + version
			break
		}
	}

	writeFile(path, strings.Join(lines, "\n"))
	return version
}

func run(cmd string) {
	args := strings.Split(cmd, " ")
	cmd, args = args[0], args[1:]

	proc := exec.Command(cmd, args...)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	if err := proc.Run(); err != nil {
		log.Fatalln(err)

	}
}

func main() {
	files := []string{"app.yaml", "api/app.yaml", "store/app.yaml", "checkout/app.yaml"}

	var version string

	for _, file := range files {
		version = bumpVersion(file)
	}

    run("git add .")
	run("git commit -m " + version)
	run("git tag " + version)
	run("git push origin master --tags")
	run("git push origin master:production")
}
