package nacha

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"
)

type NachaFile struct {
	name     string
	header   []byte
	Batches  []*Batch
	control  [][]byte
	crlf     bool
	renumber bool
}

func (nf *NachaFile) DisableCrlf() {
	nf.crlf = false
}

func (nf *NachaFile) EnableCrlf() {
	nf.crlf = true
}

func (nf *NachaFile) DisableBatchRenumber() {
	nf.renumber = false
}

func (nf *NachaFile) EnableBatchRenumber() {
	nf.renumber = true
}

func (nf *NachaFile) Name() string {
	return nf.name
}

func (nf *NachaFile) setControlTotals() {
	bCount := 0
	eCount := 0
	debitTot := 0
	creditTot := 0
	for _, batch := range nf.Batches {
		c := batch.entryCount()
		if c == 0 {
			continue
		}
		bCount += 1
		eCount += c
		dTot, cTot := batch.debitCreditTotals()
		debitTot += dTot
		creditTot += cTot
	}

	// Batch Count
	strBCount := fmt.Sprintf("%06d", bCount)
	copy(nf.control[0][1:], strBCount)

	// Entry Count
	strECount := fmt.Sprintf("%08d", eCount)
	copy(nf.control[0][13:], strECount)

	// Debit Total
	strDTot := fmt.Sprintf("%012d", debitTot)
	copy(nf.control[0][31:], strDTot)

	// Credit Total
	strCTot := fmt.Sprintf("%012d", creditTot)
	copy(nf.control[0][43:], strCTot)

	// Block Count
	records := 1 + (bCount * 2) + eCount + len(nf.control)
	blocks := (records + 9) / 10
	strBlocks := fmt.Sprintf("%06d", blocks)
	copy(nf.control[0][7:], strBlocks)

	// Entry Hash
	entryHash := int64(0)
	for _, batch := range nf.Batches {
		for _, entry := range batch.Entries {
			if entry.Remove == false {
				entryHash += entry.hash()
			}
		}
	}
	strEHash := fmt.Sprintf("%010d", entryHash)
	copy(nf.control[0][21:], strEHash[len(strEHash)-10:])
}

func (nf *NachaFile) BatchCount() int64 {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return 0
	}
	bCount, err := strconv.ParseInt(string(nf.control[0][1:7]), 10, 64)
	if err != nil {
		return 0
	}
	return bCount
}

func (nf *NachaFile) EntryCount() int64 {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return 0
	}
	bCount, err := strconv.ParseInt(string(nf.control[0][13:21]), 10, 64)
	if err != nil {
		return 0
	}
	return bCount
}

func (nf *NachaFile) DebitTotal() float64 {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return 0.00
	}
	bCount, err := strconv.ParseFloat(string(nf.control[0][31:41])+"."+string(nf.control[0][41:43]), 64)
	if err != nil {
		return 0.00
	}
	return bCount
}

func (nf *NachaFile) CreditTotal() float64 {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return 0.00
	}
	bCount, err := strconv.ParseFloat(string(nf.control[0][43:55]), 64)
	if err != nil {
		return 0.00
	}
	return bCount / 100
}

func (nf *NachaFile) BlockCount() int64 {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return 0
	}
	bCount, err := strconv.ParseInt(string(nf.control[0][7:13]), 10, 64)
	if err != nil {
		return 0
	}
	return bCount
}

func (nf *NachaFile) Hash() string {
	nf.setControlTotals()
	if len(nf.control) == 0 || len(nf.control[0]) != 94 {
		return ""
	}
	return string(nf.control[0][21:31])
}

func (nf *NachaFile) FileId() string {
	if len(nf.header) != 94 {
		return ""
	}
	return string(nf.header[33:34])
}

func (nf *NachaFile) FileCreationDate() string {
	if len(nf.header) != 94 {
		return ""
	}
	return string(nf.header[23:29])
}

func (nf *NachaFile) FileCreationTime() string {
	if len(nf.header) != 94 {
		return ""
	}
	return string(nf.header[29:33])
}

func (nf *NachaFile) FileCreationTimeStamp() (time.Time, error) {
	t, err := time.Parse("060102 1504", nf.FileCreationDate()+" "+nf.FileCreationTime())
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (nf *NachaFile) Write(w io.Writer) error {
	var batchNum int
	var crlf = "\r\n"

	records := 0

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	_, err := bw.Write(nf.header)
	if err != nil {
		return err
	}
	records++
	if nf.crlf {
		bw.WriteString(crlf)
	}

	for _, batch := range nf.Batches {

		if batch.isEmpty() {
			continue
		}
		if nf.renumber {
			batchNum++
			batch.setNumber(batchNum)
		}
		batch.setControlTotals()

		_, err = bw.Write(batch.header)
		if err != nil {
			return err
		}
		records++
		if nf.crlf {
			bw.WriteString(crlf)
		}
		for _, entry := range batch.Entries {
			if entry.Remove {
				continue
			}
			for _, data := range entry.data {
				_, err := bw.Write(data)
				if err != nil {
					return err
				}
				records++
				if nf.crlf {
					bw.WriteString(crlf)
				}
			}
		}
		_, err = bw.Write(batch.control)
		if err != nil {
			return err
		}
		records++
		if nf.crlf {
			bw.WriteString(crlf)
		}
	}
	nf.setControlTotals()
	for _, c := range nf.control {
		_, err := bw.Write(c)
		if err != nil {
			return err
		}
		records++
		if nf.crlf {
			bw.WriteString(crlf)
		}
	}

	filler := 10 - records%10
	for x := 0; x < filler; x++ {
		_, err = bw.Write(bytes.Repeat([]byte("9"), 94))
		if err != nil {
			return err
		}
		if nf.crlf {
			bw.WriteString(crlf)
		}
	}

	return nil
}

func (nf *NachaFile) WriteFile(path string) error {

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	nf.Write(f)
	return nil
}

func LoadNachaFile(name string) (*NachaFile, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	r := bufio.NewScanner(file)
	r.Split(splitScanAt(94))

	nf := &NachaFile{
		name:    name,
		control: make([][]byte, 0),
	}

	isControl := regexp.MustCompile(`[^9]`)

	var curBatch *Batch
	var curEntry *Entry

	for r.Scan() {
		d := r.Bytes()
		data := make([]byte, len(d))
		copy(data, d)

		switch data[0] {
		case '1':
			if len(nf.header) != 0 {
				return nil, errors.New("Multiple File Header Records found.")
			}
			nf.header = data
		case '5':
			if curBatch != nil && len(curBatch.control) == 0 {
				return nil, errors.New(fmt.Sprintf("Found new batch before the close of batch %d.", len(nf.Batches)))
			}
			curBatch = &Batch{
				header: data,
			}
			nf.Batches = append(nf.Batches, curBatch)
		case '6':
			if curBatch == nil {
				return nil, errors.New("Found Entry Detail Record before Batch Header Record.")
			}
			curEntry = &Entry{
				data: [][]byte{data},
			}
			curBatch.Entries = append(curBatch.Entries, curEntry)
		case '7':
			if curBatch == nil {
				return nil, errors.New("Found Entry Detail Addenda Record before Batch Header Record.")
			}
			if curEntry == nil {
				return nil, errors.New("Found Entry Detail Addenda Record before Entry Detail Record.")
			}
			curEntry.data = append(curEntry.data, data)
		case '8':
			if curBatch == nil {
				return nil, errors.New("Found Batch Control Record before Batch Header Record.")
			}
			if len(curBatch.control) != 0 {
				return nil, errors.New(fmt.Sprintf("Multiple Batch Control Records found for batch %d", len(nf.Batches)))
			}
			curBatch.control = data
		case '9':
			if isControl.Match(data) {
				nf.control = append(nf.control, data)
			}
		default:
			return nil, errors.New(fmt.Sprintf("Invalid Nacha Record Type Code. Record: %q", data))
		}
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return nf, nil
}
