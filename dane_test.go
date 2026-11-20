package stellar

import (
	"encoding/binary"
	"strings"
	"testing"
)

// answer builds a DNS response to a query: its header and question, then the
// records given, each named by a pointer to the question.
func answer(query []byte, rcode byte, records ...[]byte) []byte {
	question := query[12 : len(query)-11] // without the OPT record
	msg := append([]byte{}, query[:2]...)
	msg = append(msg, 0x81, 0x80|rcode, 0, 1)
	msg = binary.BigEndian.AppendUint16(msg, uint16(len(records)))
	msg = append(msg, 0, 0, 0, 0)
	msg = append(msg, question...)
	for _, r := range records {
		msg = append(msg, 0xc0, 12) // the question's name
		msg = append(msg, r...)
	}
	return msg
}

func record(rtype uint16, data []byte) []byte {
	out := binary.BigEndian.AppendUint16(nil, rtype)
	out = append(out, 0, 1, 0, 0, 0, 60)
	out = binary.BigEndian.AppendUint16(out, uint16(len(data)))
	return append(out, data...)
}

func TestDNSQueryAndAnswers(t *testing.T) {
	query, id, err := dnsQuery("_443._tcp.example.com", typeTLSA)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(query), "\x04_443\x04_tcp\x07example\x03com\x00\x00\x34\x00\x01") {
		t.Errorf("question = %q", query)
	}

	key := digestOf(1)
	tlsa := append([]byte{3, 1, 1}, key[:]...)
	other := append([]byte{2, 0, 1}, key[:]...)
	cname := []byte{3, 'f', 'o', 'o', 0}
	got, err := dnsAnswers(answer(query, 0, record(5, cname), record(typeTLSA, tlsa), record(typeTLSA, other)), id, typeTLSA)
	if err != nil || len(got) != 2 || string(got[0]) != string(tlsa) {
		t.Fatalf("answers = %d, %v", len(got), err)
	}

	// No such name, and a name without the type: nothing, and no error.
	for _, rcode := range []byte{0, 3} {
		if got, err := dnsAnswers(answer(query, rcode), id, typeTLSA); err != nil || len(got) != 0 {
			t.Errorf("rcode %d: %d answers, %v", rcode, len(got), err)
		}
	}
	// A failure, another query's answer, and garbage are errors.
	if _, err := dnsAnswers(answer(query, 2), id, typeTLSA); err == nil {
		t.Error("SERVFAIL is not an error")
	}
	if _, err := dnsAnswers(answer(query, 0), id+1, typeTLSA); err == nil {
		t.Error("an answer to another query is accepted")
	}
	full := answer(query, 0, record(typeTLSA, tlsa))
	for cut := 0; cut < len(full); cut++ {
		// Must not panic, whatever is cut off.
		dnsAnswers(full[:cut], id, typeTLSA)
	}
	if _, _, err := dnsQuery("a..b", typeTLSA); err == nil {
		t.Error("an empty label is accepted")
	}
}

func TestAnnouncement(t *testing.T) {
	a := Announcement{Keys: [][32]byte{digestOf(1), digestOf(2)}}
	if !a.Announces(digestOf(2)) || a.Announces(digestOf(3)) {
		t.Error("Announces is wrong")
	}
	want := "_443._tcp.example.com. IN TLSA 3 1 1 " + strings.Repeat("00", 32)
	if got := Record("example.com.", [32]byte{}); got != want {
		t.Errorf("Record = %q", got)
	}
}
