package ext4

const (
	jbdMagic      = 0xC03B3998
	jbdDescriptor = 1
	jbdCommit     = 2
	jbdSuperV1    = 3
	jbdSuperV2    = 4
	jbdRevoke     = 5

	jbdFeature64     = 0x2
	jbdFeatureCsumV2 = 0x8
	jbdFeatureCsumV3 = 0x10

	tagEscape   = 0x1
	tagSameUUID = 0x2
	tagLast     = 0x8
)

// journalCopy is one logged copy of a file system block.
type journalCopy struct {
	seq     uint32
	jblock  int64 // physical block of the copy
	escaped bool
}

// journal indexes the copies of metadata blocks still in the jbd2 log.
// The log is a ring that's only overwritten as new transactions come in,
// so it usually still holds inode table blocks from before a deletion.
type journal struct {
	copies map[int64][]journalCopy // file system block -> copies, oldest first
}
