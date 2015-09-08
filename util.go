package nacha

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// splitScanAt returns a split function that will return records of length recordLen. The records
// may optionally be terminated via new-lines, ether Dos (\r\n) or Unix(\n),
// but no termination is required. New-lines are not allowed within record. The
// function also ignores the SUB (\x1a) character if it is at EOF.
func splitScanAt(recordLen int) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(dropSub(data)) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			d := dropCR(data[0:i])
			if len(d) == recordLen {
				// We have a full newline-terminated line.
				return i + 1, d, nil
			}
			if len(d) == 0 && len(data) == i+1 {
				// We have a blank line. Okay at EOF
				return 0, nil, nil
			}
			return 0, nil, errors.New(fmt.Sprintf("Invalid record length. A new-line was found at invalid location: %q", data))
		}
		if i := len(data); i >= recordLen {
			// We have a full non-terminated line.
			return recordLen, data[0:recordLen], nil
		}
		// If we're at EOF, we have a short record
		if atEOF {
			return 0, nil, errors.New(fmt.Sprintf("Invalid record length at end-of-file. Unprocessed data: %q", data))
		}
		// Request more data.
		return 0, nil, nil
	}
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// dropSub drops a terminal \x1a from the data. This is often
// used to indicate EOF.
func dropSub(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\x1a' {
		return data[0 : len(data)-1]
	}
	return data
}

func parseCurrency(value []byte) (float64, error) {
	k := 0
	rtn := make([]byte, len(value))
	for _, chr := range value {
		if chr != ',' {
			rtn[k] = chr
			k++
		}
	}
	return strconv.ParseFloat(string(bytes.Trim(rtn[:k], " ")), 64)
}
