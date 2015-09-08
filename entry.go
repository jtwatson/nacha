package nacha

import (
	"bytes"
	"strconv"
)

type Entry struct {
	data   [][]byte
	Remove bool
}

func (e *Entry) hash() int64 {
	hash, err := strconv.ParseInt(string(e.data[0][3:11]), 10, 64)
	if err != nil {
		panic(err)
	}
	return hash
}

func (e *Entry) termId() string {
	return string(bytes.TrimRight(e.data[0][39:54], " "))
}

func (e *Entry) seq() string {
	return string(bytes.Trim(e.data[1][14:25], "0 "))
}

func (e *Entry) postAmt() (float64, error) {
	value := []byte{'0'}
	credit := false
	value = e.data[0][29:39]
	switch string(e.data[0][1:3]) {
	case "21", "22", "23", "24", "31", "32", "33", "34", "41", "42", "43", "44", "51", "52", "53", "54":
		credit = true
	}
	amt, err := parseCurrency(value)
	if err != nil {
		return 0, err
	}
	if credit {
		amt *= -1
	}
	return amt / 100, nil
}
