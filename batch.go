package nacha

import "fmt"

type SecCode string

const (
	ARC SecCode = "ARC"
	BOC SecCode = "BOC"
	CBR SecCode = "CBR"
	CCD SecCode = "CCD"
	CIE SecCode = "CIE"
	COR SecCode = "COR"
	CTX SecCode = "CTX"
	DNE SecCode = "DNE"
	IAT SecCode = "IAT"
	MTE SecCode = "MTE"
	PBR SecCode = "PBR"
	POP SecCode = "POP"
	POS SecCode = "POS"
	PPD SecCode = "PPD"
	RCK SecCode = "RCK"
	TEL SecCode = "TEL"
	WEB SecCode = "WEB"
	XCK SecCode = "XCK"
)

type Batch struct {
	header  []byte
	Entries []*Entry
	control []byte
}

func (b *Batch) isEmpty() bool {
	for _, e := range b.Entries {
		if e.Remove == false {
			return false
		}
	}
	return true
}

func (b *Batch) SecCode() SecCode {
	return SecCode(b.header[50:53])
}

func (b *Batch) setNumber(num int) {
	strNum := fmt.Sprintf("%07d", num)
	copy(b.header[87:], strNum)
	copy(b.control[87:], strNum)
}

func (b *Batch) entryCount() int {
	eCount := 0
	for _, entry := range b.Entries {
		if entry.Remove == false {
			eCount += len(entry.data)
		}
	}
	return eCount
}

func (b *Batch) debitCreditTotals() (int, int) {
	debitTot := 0
	creditTot := 0
	for _, entry := range b.Entries {
		if entry.Remove {
			continue
		}
		amt, err := entry.postAmt()
		if err != nil {
			panic(err)
		}
		if amt < 0 {
			// carefull with the rounding when converting to int.
			// Remember this is a credit
			creditTot += int((amt - 0.005) * -100)
		} else {
			debitTot += int((amt + 0.005) * 100)
		}
	}
	return debitTot, creditTot
}

func (b *Batch) setControlTotals() {

	// First set Entry / Addenda Count
	strECount := fmt.Sprintf("%06d", b.entryCount())
	copy(b.control[4:], strECount)

	// Debit / Credit Totals
	debitTot, creditTot := b.debitCreditTotals()

	// Set Debit Totals
	strDebit := fmt.Sprintf("%012d", debitTot)
	copy(b.control[20:], strDebit)

	// Set Credit Totals
	strCredit := fmt.Sprintf("%012d", creditTot)
	copy(b.control[32:], strCredit)

	// Set Hash
	var entryHash int64
	for _, entry := range b.Entries {
		if entry.Remove == false {
			entryHash += entry.hash()
		}
	}
	strEHash := fmt.Sprintf("%010d", entryHash)
	copy(b.control[10:], strEHash[len(strEHash)-10:])
}
