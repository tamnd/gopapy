package parser

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"strings"
	"sync"
	_ "embed"
)

//go:embed data/unicode_names.dat
var unicodeNamesData []byte

var (
	unicodeNamesOnce sync.Once
	unicodeNamesMap  map[string]rune
)

func lookupUnicodeName(name string) (rune, bool) {
	unicodeNamesOnce.Do(loadUnicodeNames)
	r, ok := unicodeNamesMap[strings.ToUpper(name)]
	return r, ok
}

func loadUnicodeNames() {
	gr, err := gzip.NewReader(strings.NewReader(string(unicodeNamesData)))
	if err != nil {
		return
	}
	defer gr.Close()

	var countBuf [4]byte
	if _, err := io.ReadFull(gr, countBuf[:]); err != nil {
		return
	}
	count := int(binary.BigEndian.Uint32(countBuf[:]))
	m := make(map[string]rune, count)

	var lenBuf [2]byte
	var cpBuf [4]byte
	for range count {
		if _, err := io.ReadFull(gr, lenBuf[:]); err != nil {
			return
		}
		nameLen := int(binary.BigEndian.Uint16(lenBuf[:]))
		nameBuf := make([]byte, nameLen)
		if _, err := io.ReadFull(gr, nameBuf); err != nil {
			return
		}
		cpBuf[0] = 0
		if _, err := io.ReadFull(gr, cpBuf[1:]); err != nil {
			return
		}
		cp := rune(binary.BigEndian.Uint32(cpBuf[:]))
		m[string(nameBuf)] = cp
	}
	unicodeNamesMap = m
}
