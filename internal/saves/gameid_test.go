package saves

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func ncch(programID uint64) []byte {
	b := make([]byte, 0x200)
	copy(b[0x100:], "NCCH")
	binary.LittleEndian.PutUint64(b[0x118:], programID)
	return b
}

func TestN3DSTitleIDFromNCSDAndCIA(t *testing.T) {
	const id = 0x0004000000055D00
	// NCSD with partition 0 at media unit 4 (0x800).
	ncsd := make([]byte, 0x800)
	copy(ncsd[0x100:], "NCSD")
	binary.LittleEndian.PutUint32(ncsd[0x120:], 4)
	ncsd = append(ncsd, ncch(id)...)
	got, err := N3DSTitleID(writeFile(t, "game.3ds", ncsd))
	if err != nil || got != id {
		t.Fatalf("NCSD: %x %v", got, err)
	}

	// CIA: header 0x2020, cert 0xA00, ticket 0x350, tmd 0x208 -> aligned offsets.
	cia := make([]byte, 0x20)
	binary.LittleEndian.PutUint32(cia[0x00:], 0x2020)
	binary.LittleEndian.PutUint32(cia[0x08:], 0xA00)
	binary.LittleEndian.PutUint32(cia[0x0C:], 0x350)
	binary.LittleEndian.PutUint32(cia[0x10:], 0x208)
	off := 0x2040 + 0xA00 + 0x380 + 0x240
	full := make([]byte, off)
	copy(full, cia)
	full = append(full, ncch(id)...)
	got, err = N3DSTitleID(writeFile(t, "game.cia", full))
	if err != nil || got != id {
		t.Fatalf("CIA: %x %v", got, err)
	}
}

// isoWithUMD builds a minimal ISO 9660 image whose root holds UMD_DATA.BIN.
func isoWithUMD(content string) []byte {
	const sector = 2048
	img := make([]byte, 20*sector)
	pvd := img[16*sector:]
	pvd[0] = 1
	copy(pvd[1:], "CD001")
	root := pvd[156:]
	binary.LittleEndian.PutUint32(root[2:], 18)      // root dir at sector 18
	binary.LittleEndian.PutUint32(root[10:], sector) // one sector long
	dir := img[18*sector:]
	name := "UMD_DATA.BIN;1"
	rec := dir[0:]
	rec[0] = byte(33 + len(name))
	binary.LittleEndian.PutUint32(rec[2:], 19) // file at sector 19
	binary.LittleEndian.PutUint32(rec[10:], uint32(len(content)))
	rec[32] = byte(len(name))
	copy(rec[33:], name)
	copy(img[19*sector:], content)
	return img
}

// cso compresses an image into CISO blocks of 2048 bytes.
func cso(img []byte) []byte {
	const bs = 2048
	n := (len(img) + bs - 1) / bs
	hdr := make([]byte, 24)
	copy(hdr, "CISO")
	binary.LittleEndian.PutUint32(hdr[4:], 24)
	binary.LittleEndian.PutUint64(hdr[8:], uint64(len(img)))
	binary.LittleEndian.PutUint32(hdr[16:], bs)
	index := make([]uint32, n+1)
	var data bytes.Buffer
	base := 24 + 4*(n+1)
	for i := 0; i < n; i++ {
		index[i] = uint32(base + data.Len())
		w, _ := flate.NewWriter(&data, flate.BestCompression)
		_, _ = w.Write(img[i*bs : (i+1)*bs])
		_ = w.Close()
	}
	index[n] = uint32(base + data.Len())
	out := append([]byte{}, hdr...)
	for _, v := range index {
		out = binary.LittleEndian.AppendUint32(out, v)
	}
	return append(out, data.Bytes()...)
}

func TestPSPDiscIDFromISOAndCSO(t *testing.T) {
	img := isoWithUMD("ULUS-10041|0123456789ABCDEF|0001|G")
	for name, data := range map[string][]byte{"game.iso": img, "game.cso": cso(img)} {
		got, err := PSPDiscID(writeFile(t, name, data))
		if err != nil || got != "ULUS10041" {
			t.Errorf("%s: %q %v", name, got, err)
		}
	}
}

// xtsEncryptNintendo is the inverse of xtsDecryptNintendo, for tests.
func xtsEncryptNintendo(key, data []byte, sectorSize int) []byte {
	k1, _ := aes.NewCipher(key[:16])
	k2, _ := aes.NewCipher(key[16:32])
	out := make([]byte, len(data))
	for s := 0; s*sectorSize < len(data); s++ {
		var tweak [16]byte
		binary.BigEndian.PutUint64(tweak[8:], uint64(s))
		k2.Encrypt(tweak[:], tweak[:])
		for b := 0; b < sectorSize; b += 16 {
			i := s*sectorSize + b
			var blk [16]byte
			for j := range blk {
				blk[j] = data[i+j] ^ tweak[j]
			}
			k1.Encrypt(blk[:], blk[:])
			for j := range blk {
				out[i+j] = blk[j] ^ tweak[j]
			}
			carry := byte(0)
			for j := 0; j < 16; j++ {
				next := tweak[j] >> 7
				tweak[j] = tweak[j]<<1 | carry
				carry = next
			}
			if carry != 0 {
				tweak[0] ^= 0x87
			}
		}
	}
	return out
}

func TestSwitchTitleIDFromNSP(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	keys := writeFile(t, "prod.keys", []byte("master_key_00 = 00\nheader_key = "+hex.EncodeToString(key)+"\n"))

	nca := make([]byte, 0xC00)
	copy(nca[0x200:], "NCA3")
	binary.LittleEndian.PutUint64(nca[0x210:], 0x0100000000010800) // an update ID
	encNCA := append(xtsEncryptNintendo(key, nca[:0x400], 0x200), nca[0x400:]...)

	// PFS0 with one file "a.nca".
	names := []byte("a.nca\x00")
	pfs := make([]byte, 16+0x18)
	copy(pfs, "PFS0")
	binary.LittleEndian.PutUint32(pfs[4:], 1)
	binary.LittleEndian.PutUint32(pfs[8:], uint32(len(names)))
	binary.LittleEndian.PutUint64(pfs[16+8:], uint64(len(encNCA)))
	pfs = append(append(pfs, names...), encNCA...)

	got, err := SwitchTitleID(writeFile(t, "Super Mario Odyssey.nsp", pfs), []string{keys})
	if err != nil || got != 0x0100000000010000 {
		t.Fatalf("NSP: %016X %v", got, err)
	}

	// A title ID in the file name wins and needs no keys.
	got, err = SwitchTitleID(filepath.Join(t.TempDir(), "Game [0100000000010000].xci"), nil)
	if err != nil || got != 0x0100000000010000 {
		t.Fatalf("filename tag: %016X %v", got, err)
	}
}
