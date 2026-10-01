package dos

import (
	"hash/fnv"
	"strings"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// noName is the label DOS reports for a volume without one (RBIL, INT 21h
// AX=440Dh CL=66h, "volume label or 'NO NAME    '").
const noName = "NO NAME    "

// fsType is the file system name reported for every drive: the host
// directories look like a FAT volume (RBIL: "FAT12   " or "FAT16   ").
const fsType = "FAT16   "

// initLabels encodes Config.Labels into the code page: upper case, at most
// 11 bytes (the length of a FAT volume label).
func (f *fsys) initLabels(labels map[byte]string) {
	for l, s := range labels {
		l = byte(strings.ToUpper(string(l))[0])
		if l < 'A' || l > 'Z' {
			continue
		}
		enc, _ := f.e.CP.Encode(s)
		for i, c := range enc {
			enc[i] = f.e.CP.Upper(c)
		}
		if len(enc) > 11 {
			enc = enc[:11]
		}
		f.labels[l-'A'] = strings.TrimRight(string(enc), " ")
	}
}

// serial is the volume serial number: a hash of the host root, so that it is
// stable between runs for the same directory and differs between drives.
func (f *fsys) serial(drive int) uint32 {
	h := fnv.New32a()
	h.Write([]byte(f.drives[drive]))
	return h.Sum32()
}

// labelEntry is the label as FindFirst with attribute 08h reports it: the 11
// label bytes written like an 8.3 name (a dot after the eighth character).
func (f *fsys) labelEntry(drive int) string {
	l := f.labels[drive]
	if len(l) > 8 {
		return l[:8] + "." + l[8:]
	}
	return l
}

// ioctlDrive turns a drive argument (0 = default, 1 = A:, ...) into an index;
// an unmapped drive is errBadDrive, as in MS-DOS (IOCTL.ASM, Get_Driver_BL).
func (d *DOS) ioctlDrive(n byte) (int, uint16) {
	drive := int(n) - 1
	if n == 0 {
		drive = d.fs.cur
	}
	if drive < 0 || drive >= 26 || d.fs.drives[drive] == "" {
		return 0, errBadDrive
	}
	return drive, 0
}

// mediaID writes the 440Dh/66h and 69h/00h structure at a: info level 0000h,
// binary serial number, 11 bytes of label, 8 bytes of file system type.
func (d *DOS) mediaID(a uint32, drive int) {
	label := noName
	if l := d.fs.labels[drive]; l != "" {
		label = l + strings.Repeat(" ", 11-len(l))
	}
	m := d.e.Mem
	m.W16(a, 0)
	m.W32(a+2, d.fs.serial(drive))
	m.SetBytes(a+6, []byte(label+fsType))
}

// genericIOCTL is INT 21h AX=440Dh. Only the disk category with minor code
// 66h (get media ID) is implemented; the others stop the machine.
func (d *DOS) genericIOCTL(e *hle.Env) error {
	c := e.CPU
	drive, errc := d.ioctlDrive(c.BL())
	if errc != 0 {
		d.fail(e, errc)
		return nil
	}
	if c.CH() != 0x08 { // MS-DOS: only the disk category (IOC_DC) is allowed here
		d.fail(e, errInvalidFunc)
		return nil
	}
	switch c.CL() {
	case 0x66:
		e.Note("get media ID %c:", 'A'+drive)
		d.mediaID(e.DSDX(), drive)
	default:
		return hle.Unsupported("INT 21h AX=%04Xh CX=%04Xh (generic IOCTL)", c.R[cpu.AX], c.R[cpu.CX])
	}
	d.ok(e)
	return nil
}

// getDPB is INT 21h AH=32h. The drives are host directories, not FAT
// volumes, so there is no DPB to give: the documented answer for an
// "invalid or network drive" is AL=FFh (RBIL).
func (d *DOS) getDPB(e *hle.Env) {
	if _, errc := d.ioctlDrive(e.CPU.DL()); errc == 0 {
		e.Note("no DPB: drive is a host directory")
	}
	e.CPU.SetAL(0xFF)
}
