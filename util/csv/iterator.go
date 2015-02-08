package csv

import (
	"encoding/csv"
	"os"

	"crowdstart.io/util/log"
)

type Record struct {
	Index int
	Row   []string
}

func Iterator(filename string) <-chan Record {
	ch := make(chan Record)

	go func() {
		csvfile, err := os.Open(filename)
		defer csvfile.Close()
		if err != nil {
			log.Fatal("Failed to open CSV File: %v", err)
		}

		reader := csv.NewReader(csvfile)
		reader.FieldsPerRecord = -1

		// Skip header
		reader.Read()

		// Consume CSV
		for i := 0; true; i++ {
			// Loop until exhausted
			row, err := reader.Read()

			// Break on error
			if err != nil {
				break
			}

			ch <- Record{i, row}
		}
		close(ch)
	}()
	return ch
}
