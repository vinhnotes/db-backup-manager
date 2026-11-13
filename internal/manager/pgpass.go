package manager

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Read a pgpass file independently for backup/restore connections without
// changing process-global PGPASSFILE while jobs and HTTP reads run concurrently.
func readPgpass(path, host, port, database, user string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("pgpass file must have mode 0600 or 0400")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := []string{}
		var value strings.Builder
		escaped := false
		for _, ch := range line {
			if escaped {
				value.WriteRune(ch)
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == ':' {
				fields = append(fields, value.String())
				value.Reset()
			} else {
				value.WriteRune(ch)
			}
		}
		fields = append(fields, value.String())
		if len(fields) != 5 {
			continue
		}
		match := true
		for i, want := range []string{host, port, database, user} {
			if fields[i] != "*" && fields[i] != want {
				match = false
				break
			}
		}
		if match {
			return fields[4], nil
		}
	}
	if err = scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}
