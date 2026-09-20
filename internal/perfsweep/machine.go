package perfsweep

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Machine is what the report says about where it was measured.
type Machine struct {
	CPU      string  // the processor's model name
	Cores    int     // logical CPUs the runtime sees
	MemoryGB float64 // physical memory
	OS       string  // the system and its version, kernel included
	Arch     string  // runtime.GOARCH
	Go       string  // runtime.Version()
	Commit   string  // the repository's short commit, "+dirty" with uncommitted changes
	Date     string  // YYYY-MM-DD
}

// Info collects the machine's specification, each item best effort:
// what a system does not answer stays "unknown", never a guess.
func Info(repo string) Machine {
	m := Machine{Cores: runtime.NumCPU(), Arch: runtime.GOARCH, Go: runtime.Version(), Date: time.Now().Format("2006-01-02"), CPU: "unknown", OS: runtime.GOOS}
	switch runtime.GOOS {
	case "linux":
		if v := procValue("/proc/cpuinfo", "model name"); v != "" {
			m.CPU = v
		}
		if v := procValue("/proc/meminfo", "MemTotal"); v != "" {
			if kb, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 64); err == nil {
				m.MemoryGB = kb / (1 << 20)
			}
		}
		if v := osRelease(); v != "" {
			m.OS = v
		}
		if v := run("uname", "-r"); v != "" {
			m.OS += ", kernel " + v
		}
	case "darwin":
		if v := run("sysctl", "-n", "machdep.cpu.brand_string"); v != "" {
			m.CPU = v
		}
		if v := run("sysctl", "-n", "hw.memsize"); v != "" {
			if b, err := strconv.ParseFloat(v, 64); err == nil {
				m.MemoryGB = b / (1 << 30)
			}
		}
		if v := run("sw_vers", "-productVersion"); v != "" {
			m.OS = "macOS " + v
		}
		if v := run("uname", "-r"); v != "" {
			m.OS += ", kernel " + v
		}
	}
	m.Commit = "unknown"
	if v := run("git", "-C", repo, "rev-parse", "--short", "HEAD"); v != "" {
		m.Commit = v
		if out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			m.Commit += "+dirty"
		}
	}
	return m
}

// procValue reads the first "key : value" line of a /proc file.
func procValue(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// osRelease is PRETTY_NAME of /etc/os-release.
func osRelease() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

// run returns a command's trimmed output, "" when it fails.
func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
