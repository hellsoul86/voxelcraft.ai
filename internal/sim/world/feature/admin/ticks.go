package admin

func SnapshotTick(cur uint64) uint64 {
	if cur == 0 {
		return 0
	}
	return cur - 1
}

func ArchiveTick(cur uint64) uint64 {
	if cur == 0 {
		return 0
	}
	return cur - 1
}
