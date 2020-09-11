package vimg

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/vorteil/vorteil/pkg/vio"
)

const (
	SectorSize              = 512
	GPTSignature            = 0x5452415020494645 // "EFI PART" (little-endian)
	GPTHeaderSize           = 92
	MaximumGPTEntries       = 128
	GPTEntrySize            = 128
	GPTEntriesSectors       = MaximumGPTEntries * GPTEntrySize / SectorSize
	PrimaryGPTHeaderLBA     = 1
	PrimaryGPTHeaderOffset  = SectorSize * PrimaryGPTHeaderLBA
	PrimaryGPTEntriesLBA    = PrimaryGPTHeaderLBA + 1
	PrimaryGPTEntriesOffset = SectorSize * PrimaryGPTEntriesLBA
	P0FirstLBA              = PrimaryGPTEntriesLBA + GPTEntriesSectors
	P0Offset                = P0FirstLBA * SectorSize
)

var (
	RootPartitionName = []byte{0x76, 0x0, 0x6f, 0x0, 0x72, 0x0, 0x74, 0x0, 0x65, 0x0,
		0x69, 0x0, 0x6c, 0x0, 0x2d, 0x0, 0x6f, 0x0, 0x73, 0x0} // "vorteil-os" in utf16

	DataPartitionName = []byte{0x76, 0x0, 0x6f, 0x0, 0x72, 0x0, 0x74, 0x0, 0x65, 0x0, 0x69, 0x0,
		0x6c, 0x0, 0x2d, 0x0, 0x72, 0x0, 0x6f, 0x0, 0x6f, 0x0, 0x74, 0x0} // "vorteil-root" in utf16
)

func (b *Builder) writePartitions(ctx context.Context, w io.WriteSeeker) error {

	var err error
	b.diskUID, err = b.generateUID()
	if err != nil {
		return err
	}

	err = b.writeMBR(ctx, w)
	if err != nil {
		return err
	}

	err = b.writePrimaryGPTHeader(ctx, w)
	if err != nil {
		return err
	}

	err = b.writePrimaryGPTEntries(ctx, w)
	if err != nil {
		return err
	}

	err = b.writeOS(ctx, w)
	if err != nil {
		return err
	}

	err = b.writeRoot(ctx, w)
	if err != nil {
		return err
	}

	err = b.writeSecondaryGPTEntries(ctx, w)
	if err != nil {
		return err
	}

	err = b.writeSecondaryGPTHeader(ctx, w)
	if err != nil {
		return err
	}

	return nil
}

type ProtectiveMBR struct {
	Bootloader    [446]byte
	Status        byte
	_             byte // first head
	_             byte // first sector
	_             byte // cylinder first
	PartitionType byte
	_             byte // last head
	_             byte // last sector
	_             byte // last cylinder
	FirstLBA      uint32
	TotalSectors  uint32
	_             [48]byte
	MagicNumber   [2]byte
}

func (b *Builder) writeMBR(ctx context.Context, w io.WriteSeeker) error {

	err := ctx.Err()
	if err != nil {
		return err
	}

	_, err = w.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	mbr := ProtectiveMBR{
		Status:        0x7F,
		PartitionType: 0xEE,
		FirstLBA:      1,
		MagicNumber:   [2]byte{0x55, 0xAA},
		TotalSectors:  uint32(b.size/SectorSize) - 1,
	}

	copy(mbr.Bootloader[:], Bootloader)

	err = binary.Write(w, binary.LittleEndian, &mbr)
	if err != nil {
		return err
	}

	return nil

}

type GPTHeader struct {
	Signature      uint64
	Revision       [4]byte
	HeaderSize     uint32
	CRC            uint32
	_              uint32
	CurrentLBA     uint64
	BackupLBA      uint64
	FirstUsableLBA uint64
	LastUsableLBA  uint64
	GUID           [16]byte
	StartLBAParts  uint64
	NoOfParts      uint32
	SizePartEntry  uint32
	CRCParts       uint32
	_              [420]byte
}

type GPTEntry struct {
	TypeGUID      [16]byte
	PartitionGUID [16]byte
	FirstLBA      uint64
	LastLBA       uint64
	_             uint64 // attributes
	Name          [72]byte
}

func (b *Builder) writePrimaryGPTHeader(ctx context.Context, w io.WriteSeeker) error {

	err := ctx.Err()
	if err != nil {
		return err
	}

	_, err = w.Seek(PrimaryGPTHeaderOffset, io.SeekStart)
	if err != nil {
		return err
	}

	hdr := GPTHeader{
		Signature:      GPTSignature,
		Revision:       [4]byte{0, 0, 1, 0},
		HeaderSize:     GPTHeaderSize,
		CurrentLBA:     PrimaryGPTHeaderLBA,
		BackupLBA:      uint64(b.secondaryGPTHeaderLBA),
		FirstUsableLBA: P0FirstLBA,
		LastUsableLBA:  uint64(b.lastUsableLBA),
		StartLBAParts:  2,
		NoOfParts:      MaximumGPTEntries,
		SizePartEntry:  GPTEntrySize,
		CRCParts:       b.gptEntriesCRC,
	}

	copy(hdr.GUID[:], b.diskUID)

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.LittleEndian, hdr)
	if err != nil {
		return err
	}

	crc := crc32.NewIEEE()
	_, err = io.CopyN(crc, bytes.NewReader(buf.Bytes()), GPTHeaderSize)
	if err != nil {
		return err
	}

	hdr.CRC = crc.Sum32()
	err = binary.Write(w, binary.LittleEndian, hdr)
	if err != nil {
		return err
	}

	return nil

}

func (b *Builder) writePrimaryGPTEntries(ctx context.Context, w io.WriteSeeker) error {

	err := ctx.Err()
	if err != nil {
		return err
	}

	_, err = w.Seek(PrimaryGPTEntriesOffset, io.SeekStart)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, bytes.NewReader(b.gptEntries))
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) writeSecondaryGPTHeader(ctx context.Context, w io.WriteSeeker) error {

	err := ctx.Err()
	if err != nil {
		return err
	}

	_, err = w.Seek(b.secondaryGPTHeaderOffset, io.SeekStart)
	if err != nil {
		return err
	}

	hdr := GPTHeader{
		Signature:      GPTSignature,
		Revision:       [4]byte{0, 0, 1, 0},
		HeaderSize:     GPTHeaderSize,
		CurrentLBA:     uint64(b.secondaryGPTHeaderLBA),
		BackupLBA:      PrimaryGPTHeaderLBA,
		FirstUsableLBA: P0FirstLBA,
		LastUsableLBA:  uint64(b.lastUsableLBA),
		StartLBAParts:  uint64(b.secondaryGPTEntriesLBA),
		NoOfParts:      MaximumGPTEntries,
		SizePartEntry:  GPTEntrySize,
		CRCParts:       b.gptEntriesCRC,
	}

	copy(hdr.GUID[:], b.diskUID)

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.LittleEndian, hdr)
	if err != nil {
		return err
	}

	crc := crc32.NewIEEE()
	_, err = io.CopyN(crc, bytes.NewReader(buf.Bytes()), GPTHeaderSize)
	if err != nil {
		return err
	}

	hdr.CRC = crc.Sum32()
	err = binary.Write(w, binary.LittleEndian, hdr)
	if err != nil {
		return err
	}

	return nil

}

func (b *Builder) writeSecondaryGPTEntries(ctx context.Context, w io.WriteSeeker) error {

	err := ctx.Err()
	if err != nil {
		return err
	}

	_, err = w.Seek(b.secondaryGPTEntriesOffset, io.SeekStart)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, bytes.NewReader(b.gptEntries))
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) generateUID() ([]byte, error) {

	buf := make([]byte, 16)
	_, err := io.ReadFull(b.rng, buf)
	if err != nil {
		return nil, err
	}

	// TODO: remember why I do this...
	buf[6] = buf[6]&^0xf0 | 0x40
	buf[8] = buf[8]&^0xc0 | 0x80

	return buf, nil

}

func (b *Builder) generateGPTEntries() error {

	uid0, err := b.generateUID()
	if err != nil {
		return err
	}

	p0 := GPTEntry{
		FirstLBA: uint64(b.osFirstLBA),
		LastLBA:  uint64(b.osLastLBA),
	}

	copy(p0.PartitionGUID[:], uid0)
	copy(p0.Name[:], RootPartitionName)

	uid1, err := b.generateUID()
	if err != nil {
		return err
	}

	p1 := GPTEntry{
		TypeGUID: [16]byte{0xE3, 0xBC, 0x68, 0x4F, 0xCD, 0xE8,
			0xB1, 0x4D, 0x96, 0xE7, 0xFB, 0xCA, 0xF9, 0x84, 0xB7, 0x09}, // Linux x86-64 root filesystem partition
		FirstLBA: uint64(b.rootFirstLBA),
		LastLBA:  uint64(b.rootLastLBA),
	}

	copy(p1.PartitionGUID[:], uid1)
	copy(p1.Name[:], DataPartitionName)

	entriesBuffer := new(bytes.Buffer)
	err = binary.Write(entriesBuffer, binary.LittleEndian, p0)
	if err != nil {
		return err
	}

	err = binary.Write(entriesBuffer, binary.LittleEndian, p1)
	if err != nil {
		return err
	}

	b.gptEntries = entriesBuffer.Bytes()

	crc := crc32.NewIEEE()
	_, err = io.Copy(crc, bytes.NewReader(b.gptEntries))
	if err != nil {
		return err
	}

	_, err = io.CopyN(crc, vio.Zeroes, MaximumGPTEntries*GPTEntrySize-int64(len(b.gptEntries)))
	if err != nil {
		return err
	}

	b.gptEntriesCRC = crc.Sum32()

	return nil
}
