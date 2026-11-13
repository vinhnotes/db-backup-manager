package manager

import (
	"fmt"
	"syscall"
)

func (m *Manager) checkSpace(required int64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(m.cfg.BackupDir, &st); err != nil {
		return fmt.Errorf("cannot inspect backup disk space: %w", err)
	}
	available := uint64(st.Bavail) * uint64(st.Bsize)
	const reserve = 64 * 1024 * 1024
	if required < 0 {
		required = 0
	}
	if available < uint64(required)+reserve {
		return fmt.Errorf("insufficient free disk space; keep at least 64 MiB free after writing")
	}
	return nil
}
