package searchpartial

func Partials(full string) string {
	str := ""
	for size := 3; size < len(full); size++ {
		for i := 0; i <= len(full)-size; i++ {
			str += " " + full[i:i+size]
		}
	}
	return str
}
