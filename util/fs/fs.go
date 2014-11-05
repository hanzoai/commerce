package fs

import (
	"log"
	"io/ioutil"
	"strings"
)

func ReadFile(path string) string {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	return string(dat)
}

func SplitLines(s string) []string {
	return strings.Split(s, "\n")
}

func ReadLines(path string) []string {
	return SplitLines(ReadFile(path))
}

func WriteFile(path string, data string) {
	err := ioutil.WriteFile(path, []byte(data), 0644)
	if err != nil {
		log.Fatalln(err)
	}
}
