package indiegogo

import (
	"encoding/csv"
	"os"

	"crowdstart.io/config"
	"crowdstart.io/util/log"
)

func CSVIterator(filename string) <-chan Record {
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

		for i := 0; true; i++ {
			// Only import first 25 in development
			if config.IsDevelopment && i > 25 {
				break
			}

			// Loop until exhausted
			row, err := NewRecord(reader.Read())
			if err != nil {
				break
			}

			ch <- row

		}

		close(ch)
	}()

	return ch
}
