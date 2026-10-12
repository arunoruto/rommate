package saves

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Game IDs read from ROM files, for emulators that store saves by title ID or
// disc ID instead of file name.

// ── Nintendo 3DS ─────────────────────────────────────────────────────────────

// N3DSTitleID returns the 64-bit title ID of a .3ds/.cci (NCSD), .cxi (NCCH)
// or .cia file. The NCCH header is not encrypted, so encrypted dumps work too.
func N3DSTitleID(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if strings.EqualFold(filepath.Ext(path), ".cia") {
		off, err := ciaContentOffset(f)
		if err != nil {
			return 0, err
		}
		return ncchProgramID(f, off)
	}
	// NCSD: magic at 0x100, partition 0 offset (media units of 0x200) at 0x120.
	var hdr [0x200]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil {
		return 0, err
	}
	switch string(hdr[0x100:0x104]) {
	case "NCSD":
		off := int64(binary.LittleEndian.Uint32(hdr[0x120:])) * 0x200
		return ncchProgramID(f, off)
	case "NCCH":
		return ncchProgramID(f, 0)
	}
	return 0, errors.New("not a 3DS ROM (no NCSD/NCCH header)")
}

func ncchProgramID(r io.ReaderAt, off int64) (uint64, error) {
	var hdr [0x200]byte
	if _, err := r.ReadAt(hdr[:], off); err != nil {
		return 0, err
	}
	if string(hdr[0x100:0x104]) != "NCCH" {
		return 0, errors.New("NCCH header not found")
	}
	return binary.LittleEndian.Uint64(hdr[0x118:]), nil
}

// ciaContentOffset returns where the first content (the game's NCCH) starts:
// after the header, certificate chain, ticket and TMD, each aligned to 64.
func ciaContentOffset(r io.ReaderAt) (int64, error) {
	var hdr [0x20]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return 0, err
	}
	align := func(n int64) int64 { return (n + 63) &^ 63 }
	headerSize := int64(binary.LittleEndian.Uint32(hdr[0x00:]))
	certSize := int64(binary.LittleEndian.Uint32(hdr[0x08:]))
	ticketSize := int64(binary.LittleEndian.Uint32(hdr[0x0C:]))
	tmdSize := int64(binary.LittleEndian.Uint32(hdr[0x10:]))
	return align(headerSize) + align(certSize) + align(ticketSize) + align(tmdSize), nil
}

// ── PSP ──────────────────────────────────────────────────────────────────────

// PSPDiscID returns the disc ID (e.g. "ULUS10041") of a PSP .iso or .cso, read
// from UMD_DATA.BIN in the image's root directory.
func PSPDiscID(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var img io.ReaderAt = f
	if strings.EqualFold(filepath.Ext(path), ".cso") {
		if img, err = newCSOReader(f); err != nil {
			return "", err
		}
	}
	data, err := iso9660RootFile(img, "UMD_DATA.BIN")
	if err != nil {
		return "", err
	}
	// "ULUS-10041|0123456789ABCDEF|0001|G"
	id := strings.SplitN(string(data), "|", 2)[0]
	id = strings.ReplaceAll(strings.TrimSpace(id), "-", "")
	if len(id) != 9 {
		return "", fmt.Errorf("unexpected disc ID %q", id)
	}
	return id, nil
}

// iso9660RootFile reads a file from the root directory of an ISO 9660 image.
func iso9660RootFile(r io.ReaderAt, name string) ([]byte, error) {
	const sector = 2048
	pvd := make([]byte, sector)
	if _, err := r.ReadAt(pvd, 16*sector); err != nil {
		return nil, err
	}
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return nil, errors.New("not an ISO 9660 image")
	}
	root := pvd[156:]
	rootLBA := int64(binary.LittleEndian.Uint32(root[2:]))
	rootLen := int64(binary.LittleEndian.Uint32(root[10:]))
	dir := make([]byte, rootLen)
	if _, err := r.ReadAt(dir, rootLBA*sector); err != nil {
		return nil, err
	}
	for i := 0; i < len(dir); {
		recLen := int(dir[i])
		if recLen == 0 { // records don't cross sectors; skip to the next one
			i = (i/sector + 1) * sector
			continue
		}
		rec := dir[i : i+recLen]
		nameLen := int(rec[32])
		entry := strings.SplitN(string(rec[33:33+nameLen]), ";", 2)[0]
		if strings.EqualFold(entry, name) {
			lba := int64(binary.LittleEndian.Uint32(rec[2:]))
			size := int64(binary.LittleEndian.Uint32(rec[10:]))
			out := make([]byte, size)
			_, err := r.ReadAt(out, lba*sector)
			return out, err
		}
		i += recLen
	}
	return nil, fmt.Errorf("%s not found in the image", name)
}

// csoReader reads a CISO-compressed image (blocks of raw deflate data).
type csoReader struct {
	r         io.ReaderAt
	blockSize int64
	total     int64
	align     uint
	index     []uint32
}

func newCSOReader(r io.ReaderAt) (*csoReader, error) {
	var hdr [24]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return nil, err
	}
	if string(hdr[0:4]) != "CISO" {
		return nil, errors.New("not a CSO file")
	}
	c := &csoReader{
		r:         r,
		total:     int64(binary.LittleEndian.Uint64(hdr[8:])),
		blockSize: int64(binary.LittleEndian.Uint32(hdr[16:])),
		align:     uint(hdr[21]),
	}
	if c.blockSize <= 0 {
		return nil, errors.New("invalid CSO block size")
	}
	n := (c.total+c.blockSize-1)/c.blockSize + 1
	raw := make([]byte, n*4)
	if _, err := r.ReadAt(raw, 24); err != nil {
		return nil, err
	}
	c.index = make([]uint32, n)
	for i := range c.index {
		c.index[i] = binary.LittleEndian.Uint32(raw[i*4:])
	}
	return c, nil
}

func (c *csoReader) block(i int64) ([]byte, error) {
	if i+1 >= int64(len(c.index)) {
		return nil, io.EOF
	}
	start := int64(c.index[i]&0x7FFFFFFF) << c.align
	end := int64(c.index[i+1]&0x7FFFFFFF) << c.align
	buf := make([]byte, end-start)
	if _, err := c.r.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if c.index[i]&0x80000000 != 0 { // stored uncompressed
		return buf[:min(int64(len(buf)), c.blockSize)], nil
	}
	out, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(buf)), c.blockSize))
	return out, err
}

func (c *csoReader) ReadAt(p []byte, off int64) (int, error) {
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= c.total {
			return n, io.EOF
		}
		b, err := c.block(pos / c.blockSize)
		if err != nil {
			return n, err
		}
		within := pos % c.blockSize
		if within >= int64(len(b)) {
			return n, io.ErrUnexpectedEOF
		}
		n += copy(p[n:], b[within:])
	}
	return n, nil
}

// ── Nintendo Switch ──────────────────────────────────────────────────────────

var switchTitleTag = regexp.MustCompile(`(?i)\[?(01[0-9a-f]{14})\]?`)

// SwitchTitleID returns the base title ID of a Switch game: from a
// "[0100…]" tag in the file name, or from the program ID in the first NCA of
// an .nsp/.xci, decrypted with the header key from prod.keys.
func SwitchTitleID(path string, keyFiles []string) (uint64, error) {
	if m := switchTitleTag.FindStringSubmatch(filepath.Base(path)); m != nil {
		var id uint64
		if _, err := fmt.Sscanf(m[1], "%x", &id); err == nil {
			return baseTitle(id), nil
		}
	}
	key, err := headerKey(keyFiles)
	if err != nil {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var ncaOffsets []int64
	switch strings.ToLower(filepath.Ext(path)) {
	case ".nsp":
		ncaOffsets, err = pfs0NCAs(f, 0, "PFS0", 0x18)
	case ".xci":
		ncaOffsets, err = xciNCAs(f)
	default:
		return 0, fmt.Errorf("cannot read a title ID from %s files; add the title ID to the file name, e.g. [0100…]", filepath.Ext(path))
	}
	if err != nil {
		return 0, err
	}
	for _, off := range ncaOffsets {
		if id, err := ncaProgramID(f, off, key); err == nil && id != 0 {
			return baseTitle(id), nil
		}
	}
	return 0, errors.New("no readable NCA found")
}

// baseTitle maps update/DLC IDs to the base game: the low 12 bits are 0.
func baseTitle(id uint64) uint64 { return id &^ 0xFFF }

// headerKey reads "header_key = <64 hex>" from the first readable keys file.
func headerKey(files []string) ([]byte, error) {
	for _, p := range files {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			k, v, ok := strings.Cut(sc.Text(), "=")
			if ok && strings.TrimSpace(k) == "header_key" {
				f.Close()
				b, err := hex.DecodeString(strings.TrimSpace(v))
				if err != nil || len(b) != 32 {
					return nil, fmt.Errorf("invalid header_key in %s", p)
				}
				return b, nil
			}
		}
		f.Close()
	}
	return nil, errors.New("no header_key found (needs the emulator's prod.keys); or add the title ID to the file name, e.g. [0100…]")
}

// pfs0NCAs lists the absolute offsets of .nca files in a PFS0/HFS0 partition
// at base. entrySize is 0x18 for PFS0 and 0x40 for HFS0.
func pfs0NCAs(r io.ReaderAt, base int64, magic string, entrySize int) ([]int64, error) {
	var hdr [16]byte
	if _, err := r.ReadAt(hdr[:], base); err != nil {
		return nil, err
	}
	if string(hdr[0:4]) != magic {
		return nil, fmt.Errorf("%s header not found", magic)
	}
	n := int(binary.LittleEndian.Uint32(hdr[4:]))
	strSize := int(binary.LittleEndian.Uint32(hdr[8:]))
	if n <= 0 || n > 4096 {
		return nil, errors.New("implausible partition entry count")
	}
	table := make([]byte, n*entrySize+strSize)
	if _, err := r.ReadAt(table, base+16); err != nil {
		return nil, err
	}
	strs := table[n*entrySize:]
	dataStart := base + 16 + int64(len(table))
	var out []int64
	for i := 0; i < n; i++ {
		e := table[i*entrySize:]
		off := int64(binary.LittleEndian.Uint64(e[0:]))
		nameOff := int(binary.LittleEndian.Uint32(e[16:]))
		if nameOff >= len(strs) {
			continue
		}
		name := string(strs[nameOff:])
		if j := strings.IndexByte(name, 0); j >= 0 {
			name = name[:j]
		}
		if strings.HasSuffix(strings.ToLower(name), ".nca") {
			out = append(out, dataStart+off)
		}
	}
	return out, nil
}

// xciNCAs finds the NCAs in the "secure" partition of an XCI.
func xciNCAs(r io.ReaderAt) ([]int64, error) {
	var hdr [0x200]byte
	if _, err := r.ReadAt(hdr[:], 0); err != nil {
		return nil, err
	}
	if string(hdr[0x100:0x104]) != "HEAD" {
		return nil, errors.New("not an XCI (no HEAD magic)")
	}
	rootOff := int64(binary.LittleEndian.Uint64(hdr[0x130:]))
	// The root HFS0 lists partitions (update, normal, secure, …).
	var rh [16]byte
	if _, err := r.ReadAt(rh[:], rootOff); err != nil {
		return nil, err
	}
	if string(rh[0:4]) != "HFS0" {
		return nil, errors.New("root HFS0 not found")
	}
	n := int(binary.LittleEndian.Uint32(rh[4:]))
	strSize := int(binary.LittleEndian.Uint32(rh[8:]))
	table := make([]byte, n*0x40+strSize)
	if _, err := r.ReadAt(table, rootOff+16); err != nil {
		return nil, err
	}
	strs := table[n*0x40:]
	dataStart := rootOff + 16 + int64(len(table))
	for i := 0; i < n; i++ {
		e := table[i*0x40:]
		nameOff := int(binary.LittleEndian.Uint32(e[16:]))
		name := string(strs[nameOff:])
		if j := strings.IndexByte(name, 0); j >= 0 {
			name = name[:j]
		}
		if name == "secure" {
			return pfs0NCAs(r, dataStart+int64(binary.LittleEndian.Uint64(e[0:])), "HFS0", 0x40)
		}
	}
	return nil, errors.New("secure partition not found")
}

// ncaProgramID decrypts an NCA header (AES-128-XTS, Nintendo tweak) and
// returns its program ID.
func ncaProgramID(r io.ReaderAt, off int64, key []byte) (uint64, error) {
	enc := make([]byte, 0x400) // sectors 0 and 1 hold the magic and program ID
	if _, err := r.ReadAt(enc, off); err != nil {
		return 0, err
	}
	dec, err := xtsDecryptNintendo(key, enc, 0x200)
	if err != nil {
		return 0, err
	}
	switch string(dec[0x200:0x204]) {
	case "NCA3", "NCA2":
	default:
		return 0, errors.New("NCA header did not decrypt (wrong header_key?)")
	}
	return binary.LittleEndian.Uint64(dec[0x210:]), nil
}

// xtsDecryptNintendo decrypts data in AES-128-XTS with sectors of size
// sectorSize, using Nintendo's big-endian sector-number tweak (standard XTS
// uses little-endian, so crypto/xts does not fit).
func xtsDecryptNintendo(key, data []byte, sectorSize int) ([]byte, error) {
	k1, err := aes.NewCipher(key[:16])
	if err != nil {
		return nil, err
	}
	k2, err := aes.NewCipher(key[16:32])
	if err != nil {
		return nil, err
	}
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
			k1.Decrypt(blk[:], blk[:])
			for j := range blk {
				out[i+j] = blk[j] ^ tweak[j]
			}
			// tweak *= x in GF(2^128), little-endian byte order.
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
	return out, nil
}
