package manager

import (
	"fmt"
	"math"
	"math/bits"
	"syscall"
)

// statfsBytes converts a statfs block count to bytes. The block size is
// signed on Linux, so negative values are treated as 0 and the product
// saturates instead of wrapping around on overflow.
func statfsBytes(blocks uint64, blockSize int64) uint64 {
	if blockSize <= 0 {
		return 0
	}
	hi, lo := bits.Mul64(blocks, uint64(blockSize))
	if hi != 0 {
		return math.MaxUint64
	}
	return lo
}

func (m *Manager) checkSpace(required int64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(m.cfg.BackupDir, &st); err != nil {
		return fmt.Errorf("cannot inspect backup disk space: %w", err)
	}
	available := statfsBytes(st.Bavail, int64(st.Bsize))
	const reserve = 64 * 1024 * 1024
	if required < 0 {
		required = 0
	}
	if available < uint64(required)+reserve {
		return fmt.Errorf("insufficient free disk space; keep at least 64 MiB free after writing")
	}
	return nil
}
