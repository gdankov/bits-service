package util

func PanicOnError(e error) {
	if e != nil {
		panic(e)
	}
}

var Must = PanicOnError
