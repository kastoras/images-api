package files

import (
	"bytes"
	"encoding/binary"
)

// Brands in an ISO base media "ftyp" box. http.DetectContentType recognises
// MP4 only by an "mp4*" brand, so QuickTime (.mov, e.g. from iPhones) and MP4
// files carrying only ISO brands need this check.
var (
	quickTimeBrand = []byte("qt  ")
	mp4Brands      = [][]byte{[]byte("isom"), []byte("iso2"), []byte("avc1"), []byte("mp41"), []byte("mp42"), []byte("M4V ")}
)

// sniffVideo returns video/quicktime or video/mp4 when head starts with an
// ftyp box naming one of their brands, and "" otherwise.
func sniffVideo(head []byte) string {
	brands := ftypBrands(head)
	for _, b := range brands {
		if bytes.Equal(b, quickTimeBrand) {
			return "video/quicktime"
		}
	}
	for _, b := range brands {
		for _, known := range mp4Brands {
			if bytes.Equal(b, known) {
				return "video/mp4"
			}
		}
	}
	return ""
}

// ftypBrands lists the major brand and the compatible brands of a leading
// ftyp box, skipping the minor version.
func ftypBrands(head []byte) [][]byte {
	if len(head) < 16 || !bytes.Equal(head[4:8], []byte("ftyp")) {
		return nil
	}
	size := int(binary.BigEndian.Uint32(head[:4]))
	if size < 16 || size > len(head) {
		size = len(head)
	}
	brands := [][]byte{head[8:12]}
	for at := 16; at+4 <= size; at += 4 {
		brands = append(brands, head[at:at+4])
	}
	return brands
}
